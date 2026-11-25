package xsd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/internal/w3c"
)

// TestDumpErrors zapisuje do pliku GOXSDGO_DUMP_ERRORS wszystkie błędy (schematu i dokumentu, ze wszystkimi
// polami) dla przypadków W3C w zakresie i dla korpusu. Służy do porównania wyników przed i po zmianie, która nie
// powinna zmieniać żadnego błędu, np. optymalizacji (ADR 0011): dwa zrzuty porównuje się poleceniem diff.
// Bez zmiennej test jest pomijany. REQ: DET-004
func TestDumpErrors(t *testing.T) {
	out := os.Getenv("GOXSDGO_DUMP_ERRORS")
	if out == "" {
		t.Skip("set GOXSDGO_DUMP_ERRORS=file to dump all errors")
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	line := func(id string, err error, errs []Error) {
		fmt.Fprintf(w, "%s\terr=%s\n", id, dumpErr(err))
		for _, e := range errs {
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "\t%s\n", b)
		}
	}
	schemaErrs := func(err error) []Error {
		var se *SchemaError
		if errors.As(err, &se) {
			return se.Errors
		}
		return nil
	}

	if _, err := os.Stat(w3cRoot + "/suite.xml"); err == nil {
		cases, err := w3c.Load(os.DirFS(w3cRoot), w3c.Tokens1_0)
		if err != nil {
			t.Fatal(err)
		}
		src, err := testkit.LoadSources(repoRoot)
		if err != nil {
			t.Fatal(err)
		}
		const extra = "_w3c-schemas"
		fsys := testkit.Overlay(os.DirFS(w3cRoot), extra, os.DirFS(w3cSchemas))
		cfg := Config{Locations: testkit.PrefixLocations(src.Locations("w3c-schemas"), extra), SchemaLocationHints: true}
		sets := map[string]*Set{}
		for _, c := range cases {
			if !c.InScope() {
				continue
			}
			key := strings.Join(c.SchemaDocs, "\x00")
			set, ok := sets[key]
			var cerr error
			if !ok {
				set, cerr = cfg.Compile(fsys, c.SchemaDocs...)
				sets[key] = set
			}
			if c.Kind == w3c.SchemaTest {
				line(c.ID, cerr, schemaErrs(cerr))
				continue
			}
			if set == nil {
				line(c.ID, errors.New("no schema"), nil)
				continue
			}
			r, err := fsys.Open(c.Instance)
			if err != nil {
				t.Fatal(err)
			}
			errs, verr := set.Validate(r)
			r.Close()
			line(c.ID, verr, errs)
		}
	}

	src, err := testkit.LoadSources(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(schemasDir); err == nil {
		cfg := Config{Locations: src.Locations(schemasSetName)}
		fsys := os.DirFS(schemasDir)
		var paths []string
		for _, ds := range src.Sets {
			if ds.Name == schemasSetName {
				for _, f := range ds.Files {
					if _, err := os.Stat(path.Join(schemasDir, f.Path)); err == nil {
						paths = append(paths, f.Path)
					}
				}
			}
		}
		sort.Strings(paths)
		for _, p := range paths {
			_, err := cfg.Compile(fsys, p)
			line("schema:"+p, err, schemaErrs(err))
		}
		b, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatal(err)
		}
		var expected []expectedInstance
		if err := json.Unmarshal(b, &expected); err != nil {
			t.Fatal(err)
		}
		for _, e := range expected {
			if e.Schema == "" {
				continue
			}
			set, err := cfg.Compile(fsys, e.Schema)
			if err != nil {
				line("instance:"+e.Path, err, nil)
				continue
			}
			r, err := os.Open(path.Join(instancesDir, e.Path))
			if err != nil {
				continue
			}
			errs, verr := set.Validate(r)
			r.Close()
			line("instance:"+e.Path, verr, errs)
		}
	}
}

func dumpErr(err error) string {
	if err == nil {
		return "nil"
	}
	return strings.ReplaceAll(err.Error(), "\n", " ")
}
