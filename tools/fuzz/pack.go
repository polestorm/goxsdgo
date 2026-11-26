package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Paczka dla serwera bez narzędzi Go (np. Synology NAS), ADR 0011:
//
//	fuzzpack/
//	  run.sh                       skrypt sh (BusyBox): kolejno każdy fuzzer, dziennik journal.tsv
//	  bin/<goos>-<goarch>/<pkg>.test binarki testów z `go test -c` (CGO_ENABLED=0)
//	  tree/<pakiet>/testdata/fuzz/   wejścia regresji fuzzerów, w układzie repozytorium
//	  tree/testdata/...              pobrane dane testowe, które fuzzery czytają jako wejścia startowe
//	fuzzpack.tar.gz                to samo w jednym archiwum
//
// Binarka testu uruchamiana jest w katalogu tree/<pakiet>, więc ścieżki względne z testów (testdata/fuzz,
// ../testdata/..., ../../testdata/...) działają tak samo jak w repozytorium.

// dataDirs to katalogi danych czytane przez fuzzery jako wejścia startowe (xsd/fuzzseed_test.go,
// xmlreader/fuzz_test.go, internal/regex/regex_test.go).
var dataDirs = []string{"testdata/schemas/mf", "testdata/instances/mf", "testdata/w3c/xmlconf", "testdata/w3c/xsdtests"}

func pack(root, dir string, fuzzers []fuzzer, targets []string, withData bool) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	commit, modified := gitState(root)
	if modified {
		fmt.Println("warning: uncommitted changes; runs from this package will not count towards K5")
	}
	pkgs := map[string]bool{}
	var order []string
	for _, f := range fuzzers {
		if !pkgs[f.pkg] {
			pkgs[f.pkg] = true
			order = append(order, f.pkg)
		}
	}
	for _, t := range targets {
		goos, goarch, ok := strings.Cut(strings.TrimSpace(t), "/")
		if !ok {
			return fmt.Errorf("target %q: want GOOS/GOARCH", t)
		}
		bin := filepath.Join(dir, "bin", goos+"-"+goarch)
		if err := os.MkdirAll(bin, 0o755); err != nil {
			return err
		}
		for _, p := range order {
			out, err := filepath.Abs(filepath.Join(bin, path.Base(p)+".test"))
			if err != nil {
				return err
			}
			cmd := exec.Command("go", "test", "-c", "-o", out, p)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
			if b, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("go test -c %s for %s: %v\n%s", p, t, err, b)
			}
			fmt.Printf("built %s/%s.test\n", goos+"-"+goarch, path.Base(p))
		}
	}
	tree := filepath.Join(dir, "tree")
	for _, p := range order {
		pkgDir := filepath.Join(tree, filepath.FromSlash(strings.TrimPrefix(p, "./")))
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			return err
		}
		src := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(p, "./")), "testdata", "fuzz")
		if _, err := os.Stat(src); err == nil {
			if err := copyTree(src, filepath.Join(pkgDir, "testdata", "fuzz")); err != nil {
				return err
			}
		}
	}
	if withData {
		for _, d := range dataDirs {
			src := filepath.Join(root, filepath.FromSlash(d))
			if _, err := os.Stat(src); err != nil {
				fmt.Printf("warning: %s not fetched; fuzzers will start from built-in inputs only\n", d)
				continue
			}
			if err := copyTree(src, filepath.Join(tree, filepath.FromSlash(d))); err != nil {
				return err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte(runScript(commit, fuzzers)), 0o755); err != nil {
		return err
	}
	archive := strings.TrimSuffix(filepath.Clean(dir), string(filepath.Separator)) + ".tar.gz"
	if err := tarGz(dir, archive); err != nil {
		return err
	}
	fmt.Printf("package: %s (commit %s)\narchive: %s\n", dir, commit, archive)
	return nil
}

// copyTree kopiuje drzewo plików; dowiązania do katalogów danych (w kopii roboczej) są rozwijane.
func copyTree(src, dst string) error {
	src, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

func tarGz(dir, archive string) error {
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	base := filepath.Base(filepath.Clean(dir))
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		name := path.Join(base, filepath.ToSlash(rel))
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = name
		if d.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.Type().IsRegular() {
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, in)
			in.Close()
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// runScript zwraca run.sh: zwykły sh bez rozszerzeń bash (BusyBox ash, dash).
func runScript(commit string, fuzzers []fuzzer) string {
	var list strings.Builder
	for _, f := range fuzzers {
		fmt.Fprintf(&list, "%s %s %s\n", strings.TrimPrefix(f.pkg, "./"), path.Base(f.pkg), f.name)
	}
	return `#!/bin/sh
# Kampania fuzzingu goxsdgo bez narzędzi Go (ADR 0011, docs/fuzzing.md). Commit: ` + commit + `
#
# Użycie: ./run.sh [MINUTY_ŁĄCZNIE] [WORKERY]   (domyślnie 1425 min = 23 h 45 min, workery: liczba CPU - 1)
#         WORK=/volume1/fuzz/work ./run.sh 480 2
#
# Każdy fuzzer dostaje równą część czasu. Dziennik: $WORK/journal.tsv (data, commit, pakiet, fuzzer, sekundy,
# wykonania, wynik); pełne wyjście: $WORK/logs/. Korpus fuzzerów rośnie w $WORK/cache między uruchomieniami.
# Znalezione wejście, które łamie niezmiennik, Go zapisuje w tree/<pakiet>/testdata/fuzz/<Fuzzer>/ — skopiuj je do
# repozytorium. Wiersze dziennika dopisuje do docs/fuzzing.md: go -C tools/fuzz run . -root ../.. -import journal.tsv
set -u
COMMIT=` + commit + `
BASE=$(cd "$(dirname "$0")" && pwd)
WORK=${WORK:-$BASE/work}
TOTAL=${1:-1425}
CPUS=$(nproc 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null || echo 2)
PAR=${2:-$((CPUS > 1 ? CPUS - 1 : 1))}
case "$(uname -m)" in
  x86_64|amd64) ARCH=linux-amd64 ;;
  aarch64|arm64) ARCH=linux-arm64 ;;
  *) echo "unsupported machine: $(uname -m)" >&2; exit 2 ;;
esac
mkdir -p "$WORK/logs" "$WORK/cache"
LIST='` + list.String() + `'
N=$(printf '%s' "$LIST" | grep -c .)
PER=$((TOTAL / N))
if [ "$PER" -lt 1 ]; then echo "too little time: $TOTAL min for $N fuzzers" >&2; exit 2; fi
echo "goxsdgo fuzz: $N fuzzers x $PER min, $PAR workers, $ARCH, commit $COMMIT, work $WORK"
FAILED=0
SUM_S=0
SUM_E=0
printf '%s' "$LIST" | while read -r DIR BIN NAME; do
  [ -n "$NAME" ] || continue
  START=$(date +%s)
  STAMP=$(date '+%Y-%m-%d %H:%M')
  LOG="$WORK/logs/$(date +%Y%m%d-%H%M%S)-$NAME.log"
  ( cd "$BASE/tree/$DIR" && "$BASE/bin/$ARCH/$BIN.test" -test.run '^$' -test.fuzz "^$NAME\$" \
      -test.fuzztime "${PER}m" -test.parallel "$PAR" -test.timeout 0 \
      -test.fuzzcachedir "$WORK/cache/$NAME" ) > "$LOG" 2>&1
  RC=$?
  END=$(date +%s)
  SECS=$((END - START))
  EXECS=$(sed -n 's/.*execs: \([0-9][0-9]*\).*/\1/p' "$LOG" | tail -n 1)
  [ -n "$EXECS" ] || EXECS=0
  if [ "$RC" -eq 0 ]; then
    RESULT=ok
  else
    INPUT=$(sed -n 's/.*Failing input written to \([^ ]*\).*/\1/p' "$LOG" | tail -n 1)
    RESULT="FAIL rc=$RC ${INPUT:+input=$DIR/$INPUT} log=$LOG"
  fi
  printf '%s\t%s\t./%s\t%s\t%s\t%s\t%s\n' "$STAMP" "$COMMIT" "$DIR" "$NAME" "$SECS" "$EXECS" "$RESULT" >> "$WORK/journal.tsv"
  echo "$NAME: ${SECS}s, $EXECS execs, $RESULT"
done
echo "summary (journal $WORK/journal.tsv):"
LC_ALL=C awk -F '\t' -v c="$COMMIT" '$2 == c { s += $5; e += $6; n++; if ($7 != "ok") f++ }
  END { printf "  %d runs, %.1f h, %d execs, %d failure(s)\n", n, s / 3600, e, f; exit (f > 0) }' "$WORK/journal.tsv"
`
}

// importJournal dopisuje do dziennika docs/fuzzing.md wiersze z journal.tsv z serwera.
func importJournal(src, journal, host string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	var rs []result
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		cols := strings.Split(sc.Text(), "\t")
		if len(cols) < 7 {
			continue
		}
		start, err := time.ParseInLocation("2006-01-02 15:04", cols[0], time.Local)
		if err != nil {
			return fmt.Errorf("journal: %q: %w", cols[0], err)
		}
		secs, _ := strconv.ParseInt(cols[4], 10, 64)
		execs, _ := strconv.ParseInt(cols[5], 10, 64)
		r := result{fuzzer: fuzzer{pkg: cols[2], name: cols[3]}, start: start, elapsed: time.Duration(secs) * time.Second,
			execs: execs, commit: cols[1], ok: cols[6] == "ok"}
		if !r.ok {
			r.detail = cols[6]
		}
		r.host = host
		rs = append(rs, r)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	fmt.Printf("%d row(s) appended to %s\n", len(rs), journal)
	return appendJournal(journal, rs)
}
