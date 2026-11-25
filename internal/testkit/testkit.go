// Package testkit to wspólne narzędzia testów zgodności: zapadka zaliczonych przypadków i manifest danych testowych.
//
// Zapadka (ratchet) to plik, z którego wynika, które przypadki już przechodzą. Test zgodności kończy się błędem, gdy
// któryś z nich przestaje przechodzić, a z flagą -update zapisuje plik od nowa. Liczba zaliczonych
// przypadków nie spada więc między commitami bez widocznej zmiany tego pliku (.claude/CLAUDE.md, „Jak pracujemy”).
//
// Pakiet służy tylko testom; biblioteka go nie importuje.
package testkit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Plik zapadki ma nagłówek z komentarzy („#”) i jeden identyfikator w wierszu. Wiersz „# mode: failing” oznacza, że
// lista zawiera przypadki w zakresie, które jeszcze nie przechodzą; „# mode: passing” (albo brak wiersza trybu), że
// zawiera przypadki, które już przechodzą. Ratchet zapisuje krótszą z dwóch list, więc plik zostaje mały i przy niskiej,
// i przy wysokiej zgodności.
const (
	modePassing = "passing"
	modeFailing = "failing"
)

// Ratchet porównuje wynik testu z plikiem zapadki path. inScope to identyfikatory przypadków objętych zakresem, passed
// to te, które przeszły. Przypadek uznany wcześniej za zaliczony, który teraz nie przechodzi, jest błędem testu.
// Z update=true plik jest zapisywany od nowa; inaczej test podaje, ile przypadków przechodzi po raz pierwszy.
func Ratchet(t testing.TB, path, header string, inScope, passed map[string]bool, update bool) {
	t.Helper()
	mode, ids, err := readRatchet(path)
	if err != nil {
		t.Fatal(err)
	}
	var regressed []string
	improved := 0
	for id := range inScope {
		wasPassing := ids[id] == (mode == modePassing)
		switch {
		case wasPassing && !passed[id]:
			regressed = append(regressed, id)
		case !wasPassing && passed[id]:
			improved++
		}
	}
	sort.Strings(regressed)
	if len(regressed) > 0 {
		t.Errorf("%s: %d case(s) that passed before no longer pass, first: %s", filepath.Base(path), len(regressed),
			strings.Join(regressed[:min(len(regressed), 20)], ", "))
	}
	if update {
		if err := writeRatchet(path, header, inScope, passed); err != nil {
			t.Fatal(err)
		}
		return
	}
	if improved > 0 {
		t.Logf("%s: %d newly passing case(s); record them with -update", filepath.Base(path), improved)
	}
}

func readRatchet(path string) (string, map[string]bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return modePassing, map[string]bool{}, nil
	}
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	mode := modePassing
	ids := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m, ok := strings.CutPrefix(line, "# mode: "); ok {
			if m != modePassing && m != modeFailing {
				return "", nil, fmt.Errorf("%s: unknown mode %q", path, m)
			}
			mode = m
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids[line] = true
	}
	return mode, ids, sc.Err()
}

func writeRatchet(path, header string, inScope, passed map[string]bool) error {
	var pass, fail []string
	for id := range inScope {
		if passed[id] {
			pass = append(pass, id)
		} else {
			fail = append(fail, id)
		}
	}
	mode, list := modePassing, pass
	if len(fail) < len(pass) {
		mode, list = modeFailing, fail
	}
	sort.Strings(list)
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(header), "\n") {
		b.WriteString("# " + line + "\n")
	}
	fmt.Fprintf(&b, "# in scope %d, passing %d\n# mode: %s\n", len(inScope), len(pass), mode)
	for _, id := range list {
		b.WriteString(id + "\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// Sources to manifest testdata/sources.json (ADR 0003), w części potrzebnej testom.
type Sources struct {
	Sets []struct {
		Name  string `json:"name"`
		Kind  string `json:"kind"`
		Dest  string `json:"dest"`
		Files []struct {
			Path      string   `json:"path"`
			URLs      []string `json:"urls"`
			Namespace string   `json:"namespace"`
			LocalOnly bool     `json:"local_only"`
		} `json:"files"`
	} `json:"sets"`
}

// LoadSources czyta manifest z katalogu głównego repozytorium root.
func LoadSources(root string) (*Sources, error) {
	b, err := os.ReadFile(filepath.Join(root, "testdata", "sources.json"))
	if err != nil {
		return nil, err
	}
	var s Sources
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("sources.json: %w", err)
	}
	return &s, nil
}

// Locations zwraca dla zestawu typu "files" przypisanie adresów do ścieżek w jego katalogu: katalog schematów
// dla schemaLocation w importach.
func (s *Sources) Locations(set string) map[string]string {
	m := map[string]string{}
	for _, ds := range s.Sets {
		if ds.Name != set {
			continue
		}
		for _, f := range ds.Files {
			for _, u := range f.URLs {
				m[u] = f.Path
			}
		}
	}
	return m
}

// FetchHint to komunikat dla testu pomijanego z powodu braku pobranych danych.
const FetchHint = "test data not fetched; run: go -C tools/fetch run . -root ../.. (or: python tools/fetch/fetch.py)"

// Overlay zwraca fs, w którym ścieżki zaczynające się od prefix+"/" pochodzą z extra, a pozostałe z base.
// Służy do podłączenia schematów W3C (xml.xsd, xlink.xsd) obok zestawu testów pod jednym fs.FS.
func Overlay(base fs.FS, prefix string, extra fs.FS) fs.FS {
	return overlay{base, prefix, extra}
}

type overlay struct {
	base   fs.FS
	prefix string
	extra  fs.FS
}

func (o overlay) Open(name string) (fs.File, error) {
	if rest, ok := strings.CutPrefix(name, o.prefix+"/"); ok {
		return o.extra.Open(rest)
	}
	return o.base.Open(name)
}

// PrefixLocations dopisuje prefix do każdej ścieżki w przypisaniu adresów (zob. Overlay).
func PrefixLocations(m map[string]string, prefix string) map[string]string {
	out := make(map[string]string, len(m))
	for u, p := range m {
		out[u] = prefix + "/" + p
	}
	return out
}

// SampleFiles zwraca do max plików o rozszerzeniach exts z katalogu dir (rekurencyjnie), każdy nie większy niż
// maxSize bajtów, wybranych równomiernie z posortowanej listy, więc wybór jest powtarzalny. Brak katalogu daje pustą
// listę. Służy jako wejścia startowe fuzzerów z pobranych danych testowych (ADR 0011), których nie commitujemy.
func SampleFiles(dir string, exts []string, max, maxSize int) [][]byte {
	var paths []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		for _, e := range exts {
			if strings.HasSuffix(p, e) {
				if info, err := d.Info(); err == nil && info.Size() <= int64(maxSize) {
					paths = append(paths, p)
				}
				break
			}
		}
		return nil
	})
	sort.Strings(paths)
	if len(paths) == 0 || max <= 0 {
		return nil
	}
	step := 1
	if len(paths) > max {
		step = len(paths) / max
	}
	var out [][]byte
	for i := 0; i < len(paths) && len(out) < max; i += step {
		if b, err := os.ReadFile(paths[i]); err == nil {
			out = append(out, b)
		}
	}
	return out
}
