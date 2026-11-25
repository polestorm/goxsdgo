package xsd

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/internal/w3c"
)

var update = flag.Bool("update", false, "rewrite ratchet files (lists of passing cases) and expected results")

const (
	w3cRoot    = "../testdata/w3c/xsdtests"
	w3cPassing = "../testdata/w3c/passing.txt"
	w3cSchemas = "../testdata/w3c/schemas"
)

// TestW3CSuite mierzy kryterium K1 na zestawie W3C XML Schema: każdy przypadek objęty zakresem XSD 1.0 2e
// (internal/w3c) ma dać oczekiwany werdykt. Przypadki zaliczone są zapisane w testdata/w3c/passing.txt; test
// kończy się błędem, gdy któryś z nich przestaje przechodzić. Nowe zaliczenia zapisuje `go test ./xsd -update`.
func TestW3CSuite(t *testing.T) {
	if _, err := os.Stat(w3cRoot + "/suite.xml"); err != nil {
		t.Skip(testkit.FetchHint)
	}
	cases, err := w3c.Load(os.DirFS(w3cRoot), w3c.Tokens1_0)
	if err != nil {
		t.Fatal(err)
	}
	// Schematy xml.xsd i xlink.xsd, które przypadki importują spod adresów w3.org, są w osobnym zestawie
	// manifestu; podłączamy je pod prefiksem i przypisujemy im adresy, bo biblioteka nie sięga do sieci.
	src, err := testkit.LoadSources(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	const extra = "_w3c-schemas"
	fsys := testkit.Overlay(os.DirFS(w3cRoot), extra, os.DirFS(w3cSchemas))
	cfg := Config{Locations: testkit.PrefixLocations(src.Locations("w3c-schemas"), extra)}
	passed := map[string]bool{}
	schemas := map[string]compiled{}
	for _, c := range cases {
		if !c.InScope() {
			continue
		}
		if got := runW3CCase(cfg, fsys, c, schemas); got != "" && c.Matches(got) {
			passed[c.ID] = true
		}
	}
	report := w3cReport(cases, passed)
	t.Log("\n" + report)

	list, err := testkit.ReadList(w3cPassing)
	if err != nil {
		t.Fatal(err)
	}
	if regressed := testkit.Regressions(list, passed); len(regressed) > 0 {
		t.Errorf("%d W3C case(s) from passing.txt no longer pass, first: %s", len(regressed),
			strings.Join(regressed[:min(len(regressed), 20)], ", "))
	}
	if *update {
		header := "W3C XML Schema test suite: cases goxsdgo passes (TestW3CSuite, go test ./xsd -update).\n" +
			"A case leaves this list only with an explanation in the commit message."
		if err := testkit.WriteList(w3cPassing, header, passed); err != nil {
			t.Fatal(err)
		}
	} else if n := len(passed) - len(list); n > 0 {
		t.Logf("%d newly passing case(s); record them with: go test ./xsd -run TestW3CSuite -update", n)
	}
}

type compiled struct {
	set *Set
	err error
}

// runW3CCase zwraca werdykt goxsdgo dla przypadku, albo "" gdy goxsdgo nie umie jeszcze go wydać (np. część
// rekomendacji bez implementacji): taki przypadek nigdy nie jest zaliczony.
func runW3CCase(cfg Config, fsys fs.FS, c *w3c.Case, schemas map[string]compiled) w3c.Validity {
	if len(c.SchemaDocs) == 0 {
		// Dokument wskazuje schemat przez xsi:schemaLocation; obsługa wskazówek przyjdzie w etapie 5.
		return ""
	}
	key := strings.Join(c.SchemaDocs, "\x00")
	s, ok := schemas[key]
	if !ok {
		s.set, s.err = cfg.Compile(fsys, c.SchemaDocs...)
		schemas[key] = s
	}
	if errors.Is(s.err, errors.ErrUnsupported) {
		return ""
	}
	if c.Kind == w3c.SchemaTest || s.err != nil {
		if s.err != nil {
			return w3c.Invalid
		}
		return w3c.Valid
	}
	f, err := fsys.Open(c.Instance)
	if err != nil {
		return ""
	}
	defer f.Close()
	errs, err := s.set.Validate(f)
	switch {
	case err != nil:
		return ""
	case len(errs) > 0:
		return w3c.Invalid
	}
	return w3c.Valid
}

// w3cReport składa tabelę zaliczeń według dostawców i rodzaju testu oraz liczby wyłączeń według powodów.
func w3cReport(cases []*w3c.Case, passed map[string]bool) string {
	type row struct{ schemaIn, schemaOK, instIn, instOK int }
	rows := map[string]*row{}
	excluded := map[string]int{}
	total := row{}
	for _, c := range cases {
		if !c.InScope() {
			excluded[c.Excluded]++
			continue
		}
		r := rows[c.Contributor]
		if r == nil {
			r = &row{}
			rows[c.Contributor] = r
		}
		ok := 0
		if passed[c.ID] {
			ok = 1
		}
		if c.Kind == w3c.SchemaTest {
			r.schemaIn, r.schemaOK = r.schemaIn+1, r.schemaOK+ok
			total.schemaIn, total.schemaOK = total.schemaIn+1, total.schemaOK+ok
		} else {
			r.instIn, r.instOK = r.instIn+1, r.instOK+ok
			total.instIn, total.instOK = total.instIn+1, total.instOK+ok
		}
	}
	names := make([]string, 0, len(rows))
	for n := range rows {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	line := func(name string, r row) {
		fmt.Fprintf(&b, "%-10s schema %5d/%-5d  instance %5d/%-5d  total %5d/%-5d\n", name,
			r.schemaOK, r.schemaIn, r.instOK, r.instIn, r.schemaOK+r.instOK, r.schemaIn+r.instIn)
	}
	b.WriteString("W3C XSD 1.0 2e: passed/in scope\n")
	for _, n := range names {
		line(n, *rows[n])
	}
	line("all", total)
	reasons := make([]string, 0, len(excluded))
	for r := range excluded {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		fmt.Fprintf(&b, "excluded %5d: %s\n", excluded[r], r)
	}
	return b.String()
}
