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
	w3cRatchet = "../testdata/w3c/xsdtests.ratchet"
	w3cSchemas = "../testdata/w3c/schemas"
)

// TestW3CSuite mierzy kryterium K1 na zestawie W3C XML Schema: każdy przypadek objęty zakresem XSD 1.0 2e
// (internal/w3c) ma dać oczekiwany werdykt. Zapadka testdata/w3c/xsdtests.ratchet mówi, które przypadki
// już przechodzą; test kończy się błędem, gdy któryś z nich przestaje przechodzić. Nowe zaliczenia zapisuje `go test ./xsd -update`.
// REQ: VAL-021
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
	// Oczekiwany werdykt testu schematu każdej grupy: dokument grupy z odrzuconym schematem zaliczamy jako
	// „błędny” tylko wtedy, gdy zestaw też uważa schemat za błędny.
	schemaExpected := map[string]w3c.Validity{}
	for _, c := range cases {
		if c.Kind == w3c.SchemaTest {
			schemaExpected[c.Set+"/"+c.Group] = c.Expected
		}
	}
	failures := os.Getenv("GOXSDGO_W3C_FAILURES") != ""
	for _, c := range cases {
		if !c.InScope() {
			continue
		}
		got := runW3CCase(cfg, fsys, c, schemas, schemaExpected)
		if got != "" && c.Matches(got) {
			passed[c.ID] = true
		} else if failures && c.Kind == w3c.InstanceTest {
			t.Logf("fail %s: got %q, want %s (%s)", c.ID, got, c.Expected, c.Instance)
		}
	}
	report := w3cReport(cases, passed)
	t.Log("\n" + report)

	inScope := map[string]bool{}
	for _, c := range cases {
		if c.InScope() {
			inScope[c.ID] = true
		}
	}
	testkit.Ratchet(t, w3cRatchet, "W3C XML Schema test suite, XSD 1.0 2e (TestW3CSuite, go test ./xsd -update).\n"+
		"A case stops passing only with an explanation in the commit message.", inScope, passed, *update)
}

type compiled struct {
	set *Set
	err error
}

// runW3CCase zwraca werdykt goxsdgo dla przypadku, albo "" gdy goxsdgo nie umie jeszcze go wydać (np. część
// rekomendacji bez implementacji): taki przypadek nigdy nie jest zaliczony.
func runW3CCase(cfg Config, fsys fs.FS, c *w3c.Case, schemas map[string]compiled, schemaExpected map[string]w3c.Validity) w3c.Validity {
	// Dokumenty zestawu wskazują schematy (także dodatkowe przestrzenie) przez xsi:schemaLocation, a grupy bez
	// schematu w metadanych tylko tak (ADR 0008). W teście zestawu wskazówki są więc włączone; domyślnie nie są.
	cfg.SchemaLocationHints = true
	key := strings.Join(c.SchemaDocs, "\x00")
	s, ok := schemas[key]
	if !ok {
		s.set, s.err = cfg.Compile(fsys, c.SchemaDocs...)
		schemas[key] = s
	}
	if errors.Is(s.err, errors.ErrUnsupported) {
		return ""
	}
	if c.Kind == w3c.SchemaTest {
		if s.err != nil {
			return w3c.Invalid
		}
		return w3c.Valid
	}
	if s.err != nil {
		// Schemat odrzucony: dokument nie może być poprawny, ale jako werdykt liczy się to tylko wtedy, gdy schemat
		// rzeczywiście jest błędny. Inaczej odrzucenie poprawnego schematu dawałoby fałszywe zaliczenia.
		if schemaExpected[c.Set+"/"+c.Group] == w3c.Invalid {
			return w3c.Invalid
		}
		return ""
	}
	f, err := fsys.Open(c.Instance)
	if err != nil {
		return ""
	}
	defer f.Close()
	errs, err := s.set.ValidateDocument(f, c.Instance)
	switch {
	case errors.Is(err, ErrTooManyErrors):
		return w3c.Invalid
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
	byExp := map[string][2]int{}
	for _, c := range cases {
		if !c.InScope() {
			continue
		}
		k := string(c.Kind) + " expected " + string(c.Expected)
		x := byExp[k]
		x[1]++
		if passed[c.ID] {
			x[0]++
		}
		byExp[k] = x
	}
	keys := make([]string, 0, len(byExp))
	for k := range byExp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%-28s %5d/%-5d\n", k, byExp[k][0], byExp[k][1])
	}
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
