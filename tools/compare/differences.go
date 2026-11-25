package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/polestorm/goxsdgo/internal/w3c"
)

// difference to rozbieżność werdyktu goxsdgo i libxml2 w jednym przypadku W3C.
type difference struct {
	c                *w3c.Case
	goxsdgo, libxml2 verdict
	goCode           string
	libCode          int
}

// category to kategoria rozbieżności (kryterium K3: każda różnica werdyktu opisana w docs/differences.md).
type category struct {
	key, title, rule, effect string
}

// Kategorie, gdy goxsdgo odrzuca, a libxml2 przyjmuje: według reguły zgłoszonej przez goxsdgo.
var goRejects = []struct {
	rules []string
	cat   category
}{
	{[]string{"derivation-ok-restriction", "cos-particle-restrict"}, category{"G-restriction",
		"libxml2 przyjmuje restrykcję typu złożonego niezgodną z Derivation Valid (Restriction, Complex)",
		"XSD 1, 3.4.6 (derivation-ok-restriction), 3.9.6 (Particle Valid (Restriction))",
		"schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"cos-nonambig"}, category{"G-upa",
		"libxml2 nie wykrywa części niejednoznacznych modeli zawartości",
		"XSD 1, 3.8.6 (cos-nonambig, Unique Particle Attribution), dodatek H",
		"schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"cos-element-consistent"}, category{"G-edc",
		"libxml2 nie sprawdza Element Declarations Consistent",
		"XSD 1, 3.8.6 (cos-element-consistent)",
		"schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"regex-syntax"}, category{"G-regex",
		"libxml2 przyjmuje wzorzec, który nie jest wyrażeniem regularnym XSD",
		"XSD 2, dodatek F, 4.3.4 (pattern)",
		"schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"facet-fixed"}, category{"G-facet-fixed",
		"libxml2 pozwala zmienić aspekt ustalony (fixed=\"true\") w typie bazowym",
		"XSD 2, 4.3 (składnik {fixed} każdego aspektu)",
		"schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"minLength-less-than-equal-to-maxLength", "minLength-valid-restriction", "minInclusive-less-than-maxExclusive",
		"whiteSpace-valid-restriction", "length-minLength-maxLength", "maxLength-valid-restriction", "length-valid-restriction",
		"minInclusive-less-than-equal-to-maxInclusive", "minExclusive-less-than-equal-to-maxExclusive",
		"minExclusive-less-than-maxInclusive", "totalDigits-valid-restriction", "fractionDigits-valid-restriction",
		"fractionDigits-totalDigits", "enumeration-valid-restriction", "src-single-facet-value", "enumeration-required-notation"},
		category{"G-facets", "libxml2 nie sprawdza części ograniczeń spójności aspektów przy restrykcji",
			"XSD 2, 4.3.x.4 (ograniczenia komponentów aspektów)", "schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"src-redefine"}, category{"G-redefine",
		"libxml2 przyjmuje niepoprawną redefinicję",
		"XSD 1, 4.2.2 (src-redefine)",
		"schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"sfs-attribute", "sfs-attribute-value", "sfs-attribute-required", "sfs-content", "sfs-text", "sfs-duplicate-id"}, category{"G-sfs",
		"libxml2 przyjmuje dokument schematu niezgodny ze schematem dla schematów",
		"XSD 1, dodatek A; XSD 2, dodatek A",
		"schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"sch-props-correct", "e-props-correct", "a-props-correct", "au-props-correct", "n-props-correct", "src-import",
		"src-include", "ct-props-correct", "st-props-correct", "cos-ct-extends", "src-resolve", "src-ct", "src-element",
		"src-attribute", "p-props-correct", "cos-valid-default", "mg-props-correct", "ag-props-correct", "cos-st-restricts"},
		category{"G-components", "libxml2 nie sprawdza innych ograniczeń komponentów schematu",
			"XSD 1, rozdział 3 i 4 (Schema Component Constraints, Schema Representation Constraints)",
			"schemat dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"cvc-id"}, category{"G-idref",
		"libxml2 nie sprawdza, czy wartość IDREF wskazuje ID w dokumencie",
		"XSD 1, 3.15.5 (cvc-id.1)",
		"dokument dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"cvc-pattern-valid", "cvc-datatype-valid", "cvc-enumeration-valid", "cvc-length-valid", "cvc-minLength-valid",
		"cvc-maxLength-valid", "cvc-minInclusive-valid", "cvc-maxInclusive-valid", "cvc-minExclusive-valid",
		"cvc-maxExclusive-valid", "cvc-totalDigits-valid", "cvc-fractionDigits-valid"}, category{"G-values",
		"goxsdgo odrzuca wartość typu prostego, którą libxml2 przyjmuje",
		"XSD 2, 4.1.4 (cvc-datatype-valid) i reguły aspektów 4.3.x.3",
		"dokument dotąd przyjmowany zostanie odrzucony"}},
	{[]string{"cvc-elt", "cvc-identity-constraint", "cvc-au", "cvc-complex-type", "cvc-attribute", "cvc-type"}, category{"G-assessment",
		"goxsdgo odrzuca dokument według reguły oceny, której libxml2 nie stosuje albo stosuje inaczej",
		"XSD 1, reguły cvc-* (3.3.4, 3.4.4, 3.5.4, 3.11.4)",
		"dokument dotąd przyjmowany zostanie odrzucony"}},
}

// Kategorie, gdy libxml2 odrzuca, a goxsdgo przyjmuje: według kodu libxml2.
func libRejects(kind w3c.Kind, code int) category {
	switch {
	case kind == w3c.SchemaTest:
		return category{"L-schema", "libxml2 odrzuca schemat, który goxsdgo przyjmuje",
			"zależnie od przypadku (lista niżej)", "schemat dotąd odrzucany zostanie przyjęty"}
	case code >= 1824 && code <= 1840:
		return category{"L-values", "libxml2 odrzuca wartość typu prostego, którą goxsdgo przyjmuje",
			"XSD 2, 4.1.4 i reguły aspektów", "dokument dotąd odrzucany zostanie przyjęty"}
	case code < 1000 || code >= 2000:
		return category{"L-wellformed", "libxml2 uznaje dokument za źle uformowany, a goxsdgo nie",
			"XML 1.0, Namespaces 1.0", "dokument dotąd odrzucany zostanie przyjęty"}
	}
	return category{"L-assessment", "libxml2 odrzuca dokument według reguły oceny, którą goxsdgo stosuje inaczej",
		"XSD 1, reguły cvc-*", "dokument dotąd odrzucany zostanie przyjęty"}
}

var clauseSuffix = regexp.MustCompile(`(\.[0-9]+)+$`)

func categorize(d difference) (category, bool) {
	if d.goxsdgo == invalid {
		rule := clauseSuffix.ReplaceAllString(d.goCode, "")
		for _, g := range goRejects {
			for _, r := range g.rules {
				if r == rule {
					return g.cat, true
				}
			}
		}
		return category{}, false
	}
	return libRejects(d.c.Kind, d.libCode), true
}

// whoIsRight: który walidator daje werdykt oczekiwany przez zestaw W3C.
func whoIsRight(d difference) string {
	switch {
	case d.c.Matches(w3c.Validity(d.goxsdgo)):
		return "goxsdgo"
	case d.c.Matches(w3c.Validity(d.libxml2)):
		return "libxml2"
	}
	return "żaden"
}

func libxml2Version() string {
	out, err := exec.Command("pkg-config", "--modversion", "libxml-2.0").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// writeDifferences zapisuje pełną listę rozbieżności werdyktu z kategoriami do out/compare/differences-libxml2.md,
// a z flagą -docs także do docs/differences-libxml2.md.
func writeDifferences(root, out string, docs bool, schema, instance []difference) error {
	type agg struct {
		cat            category
		n, go_, lib, x int
	}
	byCat := map[string]*agg{}
	var uncategorized []difference
	all := append(append([]difference(nil), schema...), instance...)
	cats := map[*w3c.Case]category{}
	for _, d := range all {
		c, ok := categorize(d)
		if !ok {
			uncategorized = append(uncategorized, d)
			continue
		}
		cats[d.c] = c
		a := byCat[c.key]
		if a == nil {
			a = &agg{cat: c}
			byCat[c.key] = a
		}
		a.n++
		switch whoIsRight(d) {
		case "goxsdgo":
			a.go_++
		case "libxml2":
			a.lib++
		default:
			a.x++
		}
	}
	keys := make([]string, 0, len(byCat))
	for k := range byCat {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if byCat[keys[i]].n != byCat[keys[j]].n {
			return byCat[keys[i]].n > byCat[keys[j]].n
		}
		return keys[i] < keys[j]
	})
	var b strings.Builder
	fmt.Fprintf(&b, "# Rozbieżności werdyktu goxsdgo i libxml2 na zestawie W3C\n\n")
	fmt.Fprintf(&b, "Plik generowany przez `tools/compare` (`go run -tags libxml2 . -docs w3c`); nie edytuj go ręcznie.\n")
	fmt.Fprintf(&b, "Stan: %s, libxml2 %s przez goxsdval, zestaw W3C z commitu `7bc3365c`. Kategorie i decyzje: `differences.md`.\n\n",
		time.Now().Format("2006-01-02"), libxml2Version())
	fmt.Fprintf(&b, "Rozbieżności: %d (testy schematów %d, testy dokumentów %d); bez kategorii: %d. „Rację ma” według oczekiwanego\n"+
		"werdyktu zestawu W3C.\n\n", len(all), len(schema), len(instance), len(uncategorized))
	b.WriteString("| Kategoria | Przypadki | Rację ma goxsdgo | Rację ma libxml2 | Żaden | Reguła | Skutek dla odbiorcy |\n|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		a := byCat[k]
		fmt.Fprintf(&b, "| %s: %s | %d | %d | %d | %d | %s | %s |\n", k, a.cat.title, a.n, a.go_, a.lib, a.x, a.cat.rule, a.cat.effect)
	}
	if len(uncategorized) > 0 {
		b.WriteString("\n## Bez kategorii\n\n")
		for _, d := range uncategorized {
			fmt.Fprintf(&b, "- `%s`: goxsdgo %s (%s), libxml2 %s (%d)\n", d.c.ID, d.goxsdgo, d.goCode, d.libxml2, d.libCode)
		}
	}
	b.WriteString("\n## Wszystkie przypadki\n\n| Przypadek | Status | Oczekiwane | goxsdgo | libxml2 | Kategoria | Rację ma |\n|---|---|---|---|---|---|---|\n")
	sort.Slice(all, func(i, j int) bool { return all[i].c.ID < all[j].c.ID })
	for _, d := range all {
		g := string(d.goxsdgo)
		if d.goCode != "" {
			g += " `" + d.goCode + "`"
		}
		l := string(d.libxml2)
		if d.libCode != 0 {
			l += fmt.Sprintf(" %d", d.libCode)
		}
		k := "—"
		if c, ok := cats[d.c]; ok {
			k = c.key
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s | %s |\n", d.c.ID, d.c.Status, d.c.Expected, g, l, k, whoIsRight(d))
	}
	if err := os.WriteFile(filepath.Join(out, "differences-libxml2.md"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("\nverdict differences: %d, uncategorized %d\n", len(all), len(uncategorized))
	for _, k := range keys {
		a := byCat[k]
		fmt.Printf("  %-14s %4d  goxsdgo %4d  libxml2 %3d  none %2d  %s\n", k, a.n, a.go_, a.lib, a.x, a.cat.title)
	}
	if docs {
		return os.WriteFile(filepath.Join(root, "docs", "differences-libxml2.md"), []byte(b.String()), 0o644)
	}
	return nil
}
