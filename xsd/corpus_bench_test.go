package xsd

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxsdgo/internal/testkit"
)

// BenchmarkCorpus (kryterium K7, ADR 0011) mierzy szybkość walidacji na prywatnym korpusie z katalogu
// GOXSDGO_PRIVATE_CORPUS; schemat każdego pliku wybiera przestrzeń nazw elementu głównego spośród głównych
// schematów MF z manifestu. Bez zmiennej benchmark jest pomijany.
//
// Dane prywatne (testdata/README.md): benchmark nie wypisuje nazw, ścieżek, wartości ani błędów plików — tylko liczby
// zbiorcze. Porównanie z libxml2 na tym samym korpusie: `tools/compare private`. REQ: SAFE-002, DET-006
func BenchmarkCorpus(b *testing.B) {
	dir := os.Getenv("GOXSDGO_PRIVATE_CORPUS")
	if dir == "" {
		b.Skip("set GOXSDGO_PRIVATE_CORPUS to a local directory with private XML files")
	}
	src, err := testkit.LoadSources(repoRoot)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := os.Stat(schemasDir); err != nil {
		b.Skip(testkit.FetchHint)
	}
	byNS := map[string]string{}
	for _, ds := range src.Sets {
		if ds.Name != schemasSetName {
			continue
		}
		for _, f := range ds.Files {
			if f.Namespace != "" && !strings.Contains(f.Path, "/") {
				if _, err := os.Stat(path.Join(schemasDir, f.Path)); err == nil {
					byNS[f.Namespace] = f.Path
				}
			}
		}
	}
	var files []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".xml") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	cfg := Config{Locations: src.Locations(schemasSetName)}
	sets := map[string]*Set{}
	type job struct {
		set  *Set
		file string
		size int64
	}
	var jobs []job
	var total int64
	noSchema := 0
	for _, file := range files {
		ns, err := rootNamespaceOf(file)
		if err != nil {
			continue
		}
		schema, ok := byNS[ns]
		if !ok {
			noSchema++
			continue
		}
		set, ok := sets[schema]
		if !ok {
			if set, err = cfg.Compile(os.DirFS(schemasDir), schema); err != nil {
				b.Fatalf("MF schema does not compile: %v", err)
			}
			sets[schema] = set
		}
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		jobs = append(jobs, job{set, file, info.Size()})
		total += info.Size()
	}
	b.Logf("private corpus: %d files with a schema (%.1f MB), %d without", len(jobs), float64(total)/(1<<20), noSchema)
	b.SetBytes(total)
	b.ResetTimer()
	invalid, limited := 0, 0
	for i := 0; i < b.N; i++ {
		invalid, limited = 0, 0
		for _, j := range jobs {
			f, err := os.Open(j.file)
			if err != nil {
				b.Fatal("cannot open a corpus file")
			}
			errs, err := j.set.Validate(f)
			f.Close()
			switch {
			case errors.Is(err, ErrLimit):
				limited++
			case err != nil && !errors.Is(err, ErrTooManyErrors):
				b.Fatal("validation failed with a library error")
			case len(errs) > 0:
				invalid++
			}
		}
	}
	b.StopTimer()
	b.Logf("invalid documents: %d, stopped by a limit: %d", invalid, limited)
}

func rootNamespaceOf(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := xmlreader.New(f, xmlreader.Options{})
	for {
		tok, err := r.Next()
		if err == io.EOF {
			return "", errors.New("no root element")
		}
		if err != nil {
			return "", err
		}
		if tok.Kind == xmlreader.KindStartElement {
			return tok.Name.Space, nil
		}
	}
}
