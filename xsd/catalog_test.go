package xsd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var paramFields = map[string]bool{"Document": true, "Path": true, "Name": true, "Value": true, "Facet": true, "Expected": true}

// TestCatalogMatchesSource: katalog i kod źródłowy podają ten sam zbiór kodów błędów; każdy wpis jest kompletny.
// REQ: ERR-001
func TestCatalogMatchesSource(t *testing.T) {
	src := sourceCodes(t)
	seen := map[string]bool{}
	for i, c := range catalog {
		if i > 0 && catalog[i-1].Code >= c.Code {
			t.Errorf("catalog not sorted or duplicate at %s", c.Code)
		}
		seen[c.Code] = true
		if _, ok := src[c.Code]; !ok {
			t.Errorf("catalog lists %s, which the source never emits", c.Code)
		}
		switch c.Kind {
		case KindSchema, KindDocument, KindLimit:
		default:
			t.Errorf("%s: unknown kind %q", c.Code, c.Kind)
		}
		if c.Summary == "" || !strings.HasSuffix(c.Summary, ".") {
			t.Errorf("%s: summary must be a sentence", c.Code)
		}
		if c.Spec != "" && (c.URL == "" || c.Section == "") {
			t.Errorf("%s: spec %q without section or URL", c.Code, c.Spec)
		}
		if c.Spec == "" && c.Kind != KindLimit {
			t.Errorf("%s: only library limits may have no spec", c.Code)
		}
		for _, p := range c.Params {
			if !paramFields[p] {
				t.Errorf("%s: unknown parameter %q", c.Code, p)
			}
		}
	}
	for code, at := range src {
		if !seen[code] {
			t.Errorf("code %s (first used at %s) is missing from the catalog (xsd/codes.go)", code, at)
		}
	}
	t.Logf("%d codes in the catalog", len(catalog))
}

// checkObservedCodes sprawdza kody zwrócone w testach zgodności: każdy ma wpis w katalogu, rodzaj błędu zgadza się
// z rodzajem w katalogu, a ustawione pola są wśród parametrów z katalogu.
// REQ: ERR-001, ERR-004
func checkObservedCodes(t *testing.T, o *codeObserver) {
	t.Helper()
	for code, obs := range o.codes {
		info, ok := LookupCode(code)
		if !ok {
			t.Errorf("observed code %s (%d times, e.g. %q) is not in the catalog", code, obs.count, obs.example)
			continue
		}
		for k := range obs.kinds {
			if k != info.Kind {
				t.Errorf("code %s observed as a %s error, the catalog says %s", code, k, info.Kind)
			}
		}
		declared := map[string]bool{}
		for _, p := range info.Params {
			declared[p] = true
		}
		for f := range obs.fields {
			if !declared[f] {
				t.Errorf("code %s sets Error.%s, which the catalog does not declare", code, f)
			}
		}
	}
}

// consumerCodes czyta kody libxml2, dla których odbiorca ma tłumaczenia (testdata/libxml2-consumer-codes.tsv).
func consumerCodes(t *testing.T) map[int]bool {
	t.Helper()
	f, err := os.Open("../testdata/libxml2-consumer-codes.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[int]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		n, err := strconv.Atoi(strings.SplitN(line, "\t", 2)[0])
		if err != nil {
			t.Fatalf("libxml2-consumer-codes.tsv: %v", err)
		}
		out[n] = true
	}
	return out
}

// compatOutsideConsumer to kody dokumentu, których odpowiednik libxml2 (zaobserwowany) nie ma tłumaczenia
// u odbiorcy: odbiorca pokaże dla nich komunikat ogólny. Propozycja dopisania do messages.json: ADR 0009.
var compatOutsideConsumer = map[string]bool{
	"wfc-no-external-entity-references": true, // 29
	"wfc-no-lt-in-attribute-values":     true, // 38
	"wfc-no-recursion":                  true, // 89
	"wfc-parsed-entity":                 true, // 28
	"wfc-pes-in-internal-subset":        true, // 88
	"xml-comment":                       true, // 80
	"xml-decl":                          true, // 96
	"xml-encoding-invalid":              true, // 81
}

// TestCompatTable mierzy kryterium K8: każdy kod goxsdgo poza limitami ma odpowiednik w kodach libxml2; odpowiednik
// błędu dokumentu jest kodem, który odbiorca tłumaczy (albo jawnym wyjątkiem), a błędu schematu kodem
// XML_SCHEMAP_* albo XML_SCHEMAV_*, który odbiorca tłumaczy ogólnym komunikatem dla schematu.
// REQ: ERR-006
func TestCompatTable(t *testing.T) {
	consumer := consumerCodes(t)
	used := map[int]bool{}
	for _, c := range catalog {
		m, ok := compat[c.Code]
		switch {
		case c.Kind == KindLimit:
			if ok {
				t.Errorf("%s is a limit code and must not have a libxml2 counterpart", c.Code)
			}
			continue
		case !ok:
			t.Errorf("%s has no libxml2 counterpart in the compat table", c.Code)
			continue
		}
		if !strings.HasPrefix(m.basis, "observed ") && !strings.HasPrefix(m.basis, "assigned ") {
			t.Errorf("%s: compat basis %q must start with observed or assigned", c.Code, m.basis)
		}
		if c.Kind == KindDocument {
			if !consumer[m.code] && !compatOutsideConsumer[c.Code] {
				t.Errorf("%s maps to libxml2 %d, which the consumer does not translate", c.Code, m.code)
			}
			if consumer[m.code] && compatOutsideConsumer[c.Code] {
				t.Errorf("%s is listed in compatOutsideConsumer, but libxml2 %d is translated", c.Code, m.code)
			}
			used[m.code] = true
		}
	}
	for code := range compat {
		if _, ok := LookupCode(code); !ok {
			t.Errorf("compat lists %s, which is not in the catalog", code)
		}
	}
	covered := 0
	for n := range consumer {
		if used[n] {
			covered++
		}
	}
	nonLimit := 0
	for _, c := range catalog {
		if c.Kind != KindLimit {
			nonLimit++
		}
	}
	t.Logf("compat: %d of %d codes (all except limits) have a libxml2 counterpart; %d of %d consumer codes are a counterpart of some goxsdgo document code",
		len(compat), nonLimit, covered, len(consumer))
}

// TestErrorsDoc: docs/errors.md powstaje z katalogu (go test ./xsd -run TestErrorsDoc -update).
// REQ: ERR-001
func TestErrorsDoc(t *testing.T) {
	const path = "../docs/errors.md"
	want := errorsDoc()
	if *update {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Errorf("docs/errors.md is out of date; run: go test ./xsd -run TestErrorsDoc -update")
	}
}

func errorsDoc() string {
	var b strings.Builder
	b.WriteString(`# Katalog kodów błędów

Plik generowany z ` + "`xsd/codes.go` i `xsd/compat.go`" + ` (` + "`go test ./xsd -run TestErrorsDoc -update`" + `); nie edytuj go ręcznie.
Decyzje: ` + "`docs/adr/0009-error-catalog-and-compat.md`" + `. Kody są stałe; komunikat (` + "`Error.Message`" + `) jest po angielsku
i może się zmieniać, a do tłumaczenia służy kod i parametry.

- **Rodzaj**: ` + "`schema`" + ` — błąd schematu (` + "`SchemaError.Errors`" + `); ` + "`document`" + ` — błąd dokumentu (wynik ` + "`Validate`" + `);
  ` + "`limit`" + ` — limit biblioteki albo konstrukcja nieobsługiwana, kod tylko w treści błędu zwracanego drugim wynikiem
  (` + "`ErrLimit`, `errors.ErrUnsupported`" + `).
- **Parametry**: pola ` + "`Error`" + `, które błąd może ustawić poza ` + "`Code`, `Line`, `Column`, `Message`" + `.
- **libxml2**: odpowiednik z tabeli compat (kryterium K8); „observed n/m” — ustalony na tych samych danych przez
  ` + "`tools/compare`" + `, „assigned” — przypisany, z podaną podstawą.
- **Własny**: kod nadany przez goxsdgo, bo rekomendacja nie nazywa tej reguły.

`)
	kinds := map[string]int{}
	own, withCompat := 0, 0
	for _, c := range Codes() {
		kinds[c.Kind]++
		if c.Own {
			own++
		}
		if c.LibXML2 != 0 {
			withCompat++
		}
	}
	fmt.Fprintf(&b, "Kodów: %d (schema %d, document %d, limit %d); własnych %d; z odpowiednikiem libxml2 %d.\n\n",
		len(catalog), kinds[KindSchema], kinds[KindDocument], kinds[KindLimit], own, withCompat)
	groups := []struct{ title, kind string }{
		{"Błędy dokumentu", KindDocument},
		{"Błędy schematu", KindSchema},
		{"Limity i konstrukcje nieobsługiwane", KindLimit},
	}
	for _, g := range groups {
		fmt.Fprintf(&b, "## %s\n\n| Kod | Reguła | Parametry | libxml2 | Opis |\n|---|---|---|---|---|\n", g.title)
		codes := Codes()
		sort.Slice(codes, func(i, j int) bool { return codes[i].Code < codes[j].Code })
		for _, c := range codes {
			if c.Kind != g.kind {
				continue
			}
			rule := "—"
			if c.Spec != "" {
				rule = fmt.Sprintf("[%s %s](%s)", c.Spec, c.Section, c.URL)
			}
			if c.Own {
				rule += ", własny"
			}
			lib := "—"
			if c.LibXML2 != 0 {
				lib = fmt.Sprintf("%d (%s)", c.LibXML2, c.LibXML2Basis)
			}
			params := strings.Join(c.Params, ", ")
			if params == "" {
				params = "—"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", c.Code, rule, params, lib, strings.ReplaceAll(c.Summary, "|", "\\|"))
		}
		b.WriteString("\n")
	}
	return b.String()
}
