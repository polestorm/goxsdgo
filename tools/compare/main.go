// Command compare uruchamia walidator odniesienia, libxml2 przez goxsdval, na tych samych danych co testy
// goxsdgo: zestawie W3C i korpusie MF (kryterium K3). libxml2 jest tu czarną skrzynką (docs/provenance.md).
//
// Moduł jest osobny, bo wymaga cgo i libxml2; biblioteka goxsdgo go nie importuje. Budowanie na macOS
// (libxml2 z Homebrew, co najmniej 2.15):
//
//	cd tools/compare
//	PKG_CONFIG_PATH=/opt/homebrew/opt/libxml2/lib/pkgconfig go run -tags libxml2 . w3c
//	PKG_CONFIG_PATH=/opt/homebrew/opt/libxml2/lib/pkgconfig go run -tags libxml2 . corpus
//
// Werdykty dla każdego przypadku trafiają do ../../out/compare/ (poza repozytorium), raport na wyjście.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/internal/w3c"
	"github.com/polestorm/goxsdgo/xsd"
	"github.com/polestorm/goxsdval"
)

func main() {
	root := flag.String("root", "../..", "goxsdgo repository root")
	out := flag.String("out", "", "directory for per-case verdicts (default <root>/out/compare)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: compare [-root DIR] [-out DIR] w3c|corpus")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *out == "" {
		*out = filepath.Join(*root, "out", "compare")
	}
	var err error
	switch flag.Arg(0) {
	case "w3c":
		err = runW3C(*root, *out)
	case "corpus":
		err = runCorpus(*root, *out)
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "compare:", err)
		os.Exit(1)
	}
}

// verdict to werdykt libxml2: "valid", "invalid" albo powód, dla którego przypadku nie da się porównać.
type verdict string

const (
	valid   verdict = "valid"
	invalid verdict = "invalid"
)

func comparable(v verdict) bool { return v == valid || v == invalid }

// runW3C uruchamia libxml2 na przypadkach W3C objętych zakresem i liczy zgodność z oczekiwanymi werdyktami.
func runW3C(root, out string) error {
	suite := filepath.Join(root, "testdata", "w3c", "xsdtests")
	if _, err := os.Stat(filepath.Join(suite, "suite.xml")); err != nil {
		return errors.New(testkit.FetchHint)
	}
	cases, err := w3c.Load(os.DirFS(suite), w3c.Tokens1_0)
	if err != nil {
		return err
	}
	cleanup, err := loadCatalog(root, "w3c-schemas", filepath.Join(root, "testdata", "w3c", "schemas"))
	if err != nil {
		return err
	}
	defer cleanup()
	var (
		current string
		schema  *goxsdval.Schema
		perr    error
	)
	closeSchema := func() {
		if schema != nil {
			schema.Close()
			schema = nil
		}
	}
	defer closeSchema()
	results := map[string]verdict{}
	for _, c := range cases {
		if !c.InScope() {
			continue
		}
		switch {
		case len(c.SchemaDocs) == 0:
			results[c.ID] = "not comparable: schema from xsi:schemaLocation"
			continue
		case len(c.SchemaDocs) > 1:
			// goxsdval kompiluje schemat z jednego pliku; dokument zbiorczy zmieniłby znaczenie przypadku.
			results[c.ID] = "not comparable: several schema documents"
			continue
		}
		if key := c.SchemaDocs[0]; key != current {
			closeSchema()
			current = key
			schema, perr = goxsdval.ParseSchemaFile(filepath.Join(suite, filepath.FromSlash(key)))
			if errors.Is(perr, goxsdval.ErrNotEnabled) {
				return fmt.Errorf("%w (build with -tags libxml2)", perr)
			}
		}
		if c.Kind == w3c.SchemaTest || perr != nil {
			if perr != nil {
				results[c.ID] = invalid
			} else {
				results[c.ID] = valid
			}
			continue
		}
		doc, err := os.ReadFile(filepath.Join(suite, filepath.FromSlash(c.Instance)))
		if err != nil {
			return err
		}
		errs, err := schema.Validate(doc)
		switch {
		case err != nil:
			results[c.ID] = verdict("not comparable: " + err.Error())
		case hasErrors(errs):
			results[c.ID] = invalid
		default:
			results[c.ID] = valid
		}
	}

	type row struct{ in, comparable, agree int }
	rows := map[string]*row{}
	for _, c := range cases {
		v, ok := results[c.ID]
		if !ok {
			continue
		}
		r := rows[c.Contributor]
		if r == nil {
			r = &row{}
			rows[c.Contributor] = r
		}
		r.in++
		if comparable(v) {
			r.comparable++
			if c.Matches(w3c.Validity(v)) {
				r.agree++
			}
		}
	}
	var lines []string
	for _, c := range cases {
		if v, ok := results[c.ID]; ok {
			lines = append(lines, c.ID+"\t"+string(c.Expected)+"\t"+string(v))
		}
	}
	if err := writeLines(filepath.Join(out, "libxml2-w3c.tsv"), "id\texpected\tlibxml2", lines); err != nil {
		return err
	}
	fmt.Println("libxml2 on W3C XSD 1.0 2e: agrees with expected / comparable / in scope")
	total := row{}
	names := make([]string, 0, len(rows))
	for n := range rows {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range append(names, "all") {
		r := total
		if n != "all" {
			r = *rows[n]
			total.in, total.comparable, total.agree = total.in+r.in, total.comparable+r.comparable, total.agree+r.agree
		}
		fmt.Printf("%-10s %6d / %6d / %6d  (%.1f%% of comparable)\n", n, r.agree, r.comparable, r.in,
			100*float64(r.agree)/float64(max(r.comparable, 1)))
	}
	return compareSchemaTests(root, out, cases, results)
}

// compareSchemaTests porównuje werdykty testów schematów: goxsdgo (xsd.Compile) i libxml2 z oczekiwanymi i ze
// sobą (kryterium K3 dla schematów, od etapu 3). Rozbieżności goxsdgo z libxml2 trafiają do pliku.
func compareSchemaTests(root, out string, cases []*w3c.Case, libxml2 map[string]verdict) error {
	src, err := testkit.LoadSources(root)
	if err != nil {
		return err
	}
	const extra = "_w3c-schemas"
	suite := filepath.Join(root, "testdata", "w3c", "xsdtests")
	fsys := testkit.Overlay(os.DirFS(suite), extra, os.DirFS(filepath.Join(root, "testdata", "w3c", "schemas")))
	cfg := xsd.Config{Locations: testkit.PrefixLocations(src.Locations("w3c-schemas"), extra)}
	type row struct{ n, goOK, libOK, same int }
	rows := map[string]*row{}
	var diffs []string
	for _, c := range cases {
		lv := libxml2[c.ID]
		if c.Kind != w3c.SchemaTest || !comparable(lv) {
			continue
		}
		_, err := cfg.Compile(fsys, c.SchemaDocs...)
		gv, code := valid, ""
		switch {
		case errors.Is(err, errors.ErrUnsupported):
			continue
		case err != nil:
			gv = invalid
			var se *xsd.SchemaError
			if errors.As(err, &se) && len(se.Errors) > 0 {
				code = se.Errors[0].Code
			}
		}
		r := rows[c.Contributor]
		if r == nil {
			r = &row{}
			rows[c.Contributor] = r
		}
		r.n++
		if c.Matches(w3c.Validity(gv)) {
			r.goOK++
		}
		if c.Matches(w3c.Validity(lv)) {
			r.libOK++
		}
		if gv == lv {
			r.same++
		} else {
			diffs = append(diffs, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", c.ID, c.Expected, c.Status, gv, lv, code))
		}
	}
	if err := writeLines(filepath.Join(out, "schemas-goxsdgo-libxml2.tsv"), "id\texpected\tstatus\tgoxsdgo\tlibxml2\tgoxsdgo-code", diffs); err != nil {
		return err
	}
	fmt.Println("\nW3C schema tests, comparable: goxsdgo correct / libxml2 correct / same verdict / cases")
	names := make([]string, 0, len(rows))
	for n := range rows {
		names = append(names, n)
	}
	sort.Strings(names)
	total := row{}
	for _, n := range append(names, "all") {
		r := total
		if n != "all" {
			r = *rows[n]
			total.n, total.goOK, total.libOK, total.same = total.n+r.n, total.goOK+r.goOK, total.libOK+r.libOK, total.same+r.same
		}
		fmt.Printf("%-10s %6d / %6d / %6d / %6d  (same verdict %.1f%%)\n", n, r.goOK, r.libOK, r.same, r.n,
			100*float64(r.same)/float64(max(r.n, 1)))
	}
	return nil
}

// runCorpus kompiluje każdy schemat MF i waliduje dokumenty z testdata/instances/expected.json; importy
// rozwiązuje katalog XML zbudowany z manifestu, więc libxml2 nie sięga do sieci.
func runCorpus(root, out string) error {
	src, err := testkit.LoadSources(root)
	if err != nil {
		return err
	}
	schemas := filepath.Join(root, "testdata", "schemas", "mf")
	if _, err := os.Stat(schemas); err != nil {
		return errors.New(testkit.FetchHint)
	}
	abs, err := filepath.Abs(schemas)
	if err != nil {
		return err
	}
	cleanup, err := loadCatalog(root, "mf-schemas", schemas)
	if err != nil {
		return err
	}
	defer cleanup()

	var lines []string
	paths := make([]string, 0)
	for _, ds := range src.Sets {
		if ds.Name == "mf-schemas" {
			for _, f := range ds.Files {
				paths = append(paths, f.Path)
			}
		}
	}
	sort.Strings(paths)
	ok := 0
	for _, p := range paths {
		s, err := goxsdval.ParseSchemaFile(filepath.Join(abs, filepath.FromSlash(p)))
		if err != nil {
			lines = append(lines, "schema:"+p+"\tinvalid\t"+oneLine(err.Error()))
			fmt.Printf("schema %s: %v\n", p, err)
			continue
		}
		s.Close()
		ok++
		lines = append(lines, "schema:"+p+"\tvalid\t")
	}
	fmt.Printf("MF schemas compiled by libxml2: %d/%d\n", ok, len(paths))

	expected, err := readExpected(filepath.Join(root, "testdata", "instances", "expected.json"))
	if err != nil {
		return err
	}
	agree, checked := 0, 0
	for _, e := range expected {
		if e.Schema == "" {
			continue
		}
		checked++
		s, err := goxsdval.ParseSchemaFile(filepath.Join(abs, filepath.FromSlash(e.Schema)))
		if err != nil {
			fmt.Printf("instance %s: schema %s: %v\n", e.Path, e.Schema, err)
			continue
		}
		doc, err := os.ReadFile(filepath.Join(root, "testdata", "instances", filepath.FromSlash(e.Path)))
		if err != nil {
			s.Close()
			return err
		}
		errs, err := s.Validate(doc)
		s.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", e.Path, err)
		}
		got := !hasErrors(errs)
		detail := ""
		if !got {
			first := firstError(errs)
			detail = fmt.Sprintf("line %d: %d %s", first.Line, first.Code, oneLine(first.Message))
		}
		if got == e.Valid {
			agree++
		} else {
			fmt.Printf("instance %s: libxml2 valid=%v, expected %v; %s\n", e.Path, got, e.Valid, detail)
		}
		lines = append(lines, fmt.Sprintf("instance:%s\t%v\t%s", e.Path, got, detail))
	}
	fmt.Printf("MF instances where libxml2 gives the expected verdict: %d/%d\n", agree, checked)
	return writeLines(filepath.Join(out, "libxml2-corpus.tsv"), "id\tlibxml2\tdetail", lines)
}

// loadCatalog rejestruje w libxml2 katalog XML dla zestawu set z manifestu, którego pliki leżą w dir. libxml2
// czyta plik katalogu dopiero przy pierwszym imporcie, więc usuwa go dopiero zwrócona funkcja, na końcu przebiegu.
func loadCatalog(root, set, dir string) (func(), error) {
	src, err := testkit.LoadSources(root)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, errors.New(testkit.FetchHint)
	}
	catalog, err := writeCatalog(src.Locations(set), abs)
	if err != nil {
		return nil, err
	}
	cleanup := func() { os.Remove(catalog) }
	if err := goxsdval.LoadCatalogFile(catalog); err != nil {
		cleanup()
		return nil, err
	}
	return cleanup, nil
}

func hasErrors(errs []goxsdval.ValidationError) bool {
	for _, e := range errs {
		if e.Severity >= goxsdval.SeverityError {
			return true
		}
	}
	return false
}

func firstError(errs []goxsdval.ValidationError) goxsdval.ValidationError {
	for _, e := range errs {
		if e.Severity >= goxsdval.SeverityError {
			return e
		}
	}
	return goxsdval.ValidationError{}
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func writeLines(path, header string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(header+"\n"+strings.Join(lines, "\n")+"\n"), 0o644)
}
