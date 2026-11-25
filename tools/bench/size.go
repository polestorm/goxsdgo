package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Rozmiar programu: najmniejszy program, który kompiluje schemat z pliku i waliduje dokument z pliku
// (tools/bench/sizeprog/<kandydat>), zbudowany z CGO_ENABLED=0 i -ldflags="-s -w", minus ten sam program bez
// walidatora (sizeprog/none). Linie kodu: pliki .go (bez testów) pakietów spoza biblioteki standardowej, które
// program ściąga, bez samego programu — jak w gomdgo.
type sizeResult struct {
	Candidate  string `json:"candidate"`
	Bytes      int64  `json:"bytes"`
	AddedBytes int64  `json:"added_bytes"`
	Lines      int    `json:"lines"`
	Modules    int    `json:"modules"` // moduły poza stdlib i samym kandydatem
	Note       string `json:"note,omitempty"`
}

var sizeProgs = []struct{ dir, name string }{
	{"goxsdgo", "goxsdgo"}, {"knroy", "knroy/go-xml"}, {"jacoelho", "jacoelho/xsd"}, {"cbsigma", "cB-Sigma/xsd"},
}

func runSize(root, out string) error {
	bench := filepath.Join(root, "tools", "bench")
	tmp, err := os.MkdirTemp("", "goxsdgo-size-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	build := func(dir string) (int64, error) {
		bin := filepath.Join(tmp, dir)
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", bin, "./sizeprog/"+dir)
		cmd.Dir = bench
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if b, err := cmd.CombinedOutput(); err != nil {
			return 0, fmt.Errorf("%s: %v\n%s", dir, err, b)
		}
		st, err := os.Stat(bin)
		if err != nil {
			return 0, err
		}
		return st.Size(), nil
	}
	base, err := build("none")
	if err != nil {
		return err
	}
	var all []sizeResult
	for _, p := range sizeProgs {
		n, err := build(p.dir)
		if err != nil {
			return err
		}
		lines, mods, err := countLines(bench, "./sizeprog/"+p.dir)
		if err != nil {
			return err
		}
		r := sizeResult{Candidate: p.name, Bytes: n, AddedBytes: n - base, Lines: lines, Modules: mods}
		fmt.Printf("%-14s binary %6.0f kB, added %6.0f kB, %6d lines, %d other modules\n", r.Candidate,
			float64(r.Bytes)/1000, float64(r.AddedBytes)/1000, r.Lines, r.Modules)
		all = append(all, r)
	}
	return writeJSON(out, "size.json", all)
}

// countLines liczy wiersze plików Go (bez testów) pakietów spoza stdlib, które ściąga program, bez niego
// samego; zwraca też liczbę modułów innych niż moduł kandydata i bench.
func countLines(dir, pkg string) (int, int, error) {
	cmd := exec.Command("go", "list", "-deps", "-f",
		"{{if not .Standard}}{{.ImportPath}}|{{if .Module}}{{.Module.Path}}{{end}}|{{.Dir}}|{{join .GoFiles \",\"}}{{end}}", pkg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	b, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	total := 0
	mods := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 4 || strings.Contains(parts[0], "/tools/bench") {
			continue
		}
		mods[parts[1]] = true
		for _, f := range strings.Split(parts[3], ",") {
			if f == "" {
				continue
			}
			n, err := lineCount(filepath.Join(parts[2], f))
			if err != nil {
				return 0, 0, err
			}
			total += n
		}
	}
	return total, len(mods) - 1, nil
}

// lineCount liczy wiersze pliku (tylko liczba, treści nie przetwarzamy).
func lineCount(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}
