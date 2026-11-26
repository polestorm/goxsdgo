package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// publishedFiles: wyniki z out/bench, które publish kopiuje do docs/results bez zmian.
var publishedFiles = []string{"w3c.json", "contested.json", "corpus.json", "perf.json", "size.json", "probes.txt",
	"environment-w3c.json", "environment-corpus.json", "environment-perf.json", "environment-probe.json",
	"environment-size.json"}

// runPublish składa surowe wyniki w docs/results: werdykt każdego kandydata dla każdego przypadku W3C w jednym
// pliku (w3c-verdicts.tsv) oraz pliki JSON z out/bench, żeby liczby w README i docs/comparison.md dało się
// sprawdzić bez ponownego uruchamiania pomiarów.
func runPublish(root, out string) error {
	dst := filepath.Join(root, "docs", "results")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	var cols []*verdicts
	var expected map[string]string
	var ids []string
	for _, cf := range candidateFiles {
		path := filepath.Join(root, "out", filepath.FromSlash(cf.File))
		v, err := readVerdicts(cf.Name, path)
		if err != nil {
			return fmt.Errorf("%s: %w (run w3c first; libxml2 via tools/compare)", cf.Name, err)
		}
		cols = append(cols, v)
		if expected == nil {
			expected, ids, err = readExpected(path)
			if err != nil {
				return err
			}
		}
	}
	var b strings.Builder
	b.WriteString("id\texpected")
	for _, v := range cols {
		b.WriteString("\t" + v.Name)
	}
	b.WriteString("\n")
	for _, id := range ids {
		b.WriteString(id + "\t" + expected[id])
		for _, v := range cols {
			got := v.ByID[id]
			if got == "" {
				got = "-"
			}
			b.WriteString("\t" + got)
		}
		b.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(dst, "w3c-verdicts.tsv"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	var missing []string
	for _, name := range publishedFiles {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			missing = append(missing, name)
			continue
		}
		data = []byte(scrubPaths(string(data), root))
		if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
			return err
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		fmt.Println("not published (run the mode first):", strings.Join(missing, ", "))
	}
	fmt.Printf("docs/results: %d W3C cases × %d candidates\n", len(ids), len(cols))
	return nil
}

var tmpPath = regexp.MustCompile(`(/private)?/var/folders/[^/]+/[^/]+/T/`)

// scrubPaths usuwa z publikowanych wyników ścieżki lokalnej maszyny: katalog repozytorium zamienia na "<root>",
// katalog tymczasowy na "<tmp>/".
func scrubPaths(s, root string) string {
	s = strings.ReplaceAll(s, root, "<root>")
	if t := os.TempDir(); len(t) > 1 {
		s = strings.ReplaceAll(s, strings.TrimSuffix(t, "/")+"/", "<tmp>/")
	}
	return tmpPath.ReplaceAllString(s, "<tmp>/")
}

// readExpected czyta kolumny id i expected z pliku werdyktów, w kolejności pliku.
func readExpected(path string) (map[string]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	exp := map[string]string{}
	var ids []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		parts := strings.Split(sc.Text(), "\t")
		if len(parts) >= 2 {
			if _, dup := exp[parts[0]]; !dup {
				ids = append(ids, parts[0])
			}
			exp[parts[0]] = parts[1]
		}
	}
	return exp, ids, sc.Err()
}
