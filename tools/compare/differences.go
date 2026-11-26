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
		"libxml2 accepts a complex type restriction that violates Derivation Valid (Restriction, Complex)",
		"XSD 1, 3.4.6 (derivation-ok-restriction), 3.9.6 (Particle Valid (Restriction))",
		"a schema accepted so far will be rejected"}},
	{[]string{"cos-nonambig"}, category{"G-upa",
		"libxml2 misses some ambiguous content models",
		"XSD 1, 3.8.6 (cos-nonambig, Unique Particle Attribution), appendix H",
		"a schema accepted so far will be rejected"}},
	{[]string{"cos-element-consistent"}, category{"G-edc",
		"libxml2 does not check Element Declarations Consistent",
		"XSD 1, 3.8.6 (cos-element-consistent)",
		"a schema accepted so far will be rejected"}},
	{[]string{"regex-syntax"}, category{"G-regex",
		"libxml2 accepts a pattern that is not an XSD regular expression",
		"XSD 2, appendix F, 4.3.4 (pattern)",
		"a schema accepted so far will be rejected"}},
	{[]string{"facet-fixed"}, category{"G-facet-fixed",
		"libxml2 lets a restriction change a facet fixed (fixed=\"true\") in the base type",
		"XSD 2, 4.3 (the {fixed} property of every facet)",
		"a schema accepted so far will be rejected"}},
	{[]string{"minLength-less-than-equal-to-maxLength", "minLength-valid-restriction", "minInclusive-less-than-maxExclusive",
		"whiteSpace-valid-restriction", "length-minLength-maxLength", "maxLength-valid-restriction", "length-valid-restriction",
		"minInclusive-less-than-equal-to-maxInclusive", "minExclusive-less-than-equal-to-maxExclusive",
		"minExclusive-less-than-maxInclusive", "totalDigits-valid-restriction", "fractionDigits-valid-restriction",
		"fractionDigits-totalDigits", "enumeration-valid-restriction", "src-single-facet-value", "enumeration-required-notation"},
		category{"G-facets", "libxml2 does not check some facet consistency constraints of a restriction",
			"XSD 2, 4.3.x.4 (facet component constraints)", "a schema accepted so far will be rejected"}},
	{[]string{"src-redefine"}, category{"G-redefine",
		"libxml2 accepts an invalid redefinition",
		"XSD 1, 4.2.2 (src-redefine)",
		"a schema accepted so far will be rejected"}},
	{[]string{"sfs-attribute", "sfs-attribute-value", "sfs-attribute-required", "sfs-content", "sfs-text", "sfs-duplicate-id"}, category{"G-sfs",
		"libxml2 accepts a schema document that violates the schema for schemas",
		"XSD 1, appendix A; XSD 2, appendix A",
		"a schema accepted so far will be rejected"}},
	{[]string{"sch-props-correct", "e-props-correct", "a-props-correct", "au-props-correct", "n-props-correct", "src-import",
		"src-include", "ct-props-correct", "st-props-correct", "cos-ct-extends", "src-resolve", "src-ct", "src-element",
		"src-attribute", "p-props-correct", "cos-valid-default", "mg-props-correct", "ag-props-correct", "cos-st-restricts"},
		category{"G-components", "libxml2 does not check other schema component constraints",
			"XSD 1, chapters 3 and 4 (Schema Component Constraints, Schema Representation Constraints)",
			"a schema accepted so far will be rejected"}},
	{[]string{"cvc-id"}, category{"G-idref",
		"libxml2 does not check that an IDREF value points to an ID in the document",
		"XSD 1, 3.15.5 (cvc-id.1)",
		"a document accepted so far will be rejected"}},
	{[]string{"cvc-pattern-valid", "cvc-datatype-valid", "cvc-enumeration-valid", "cvc-length-valid", "cvc-minLength-valid",
		"cvc-maxLength-valid", "cvc-minInclusive-valid", "cvc-maxInclusive-valid", "cvc-minExclusive-valid",
		"cvc-maxExclusive-valid", "cvc-totalDigits-valid", "cvc-fractionDigits-valid"}, category{"G-values",
		"goxsdgo rejects a simple-type value that libxml2 accepts",
		"XSD 2, 4.1.4 (cvc-datatype-valid) and the facet rules 4.3.x.3",
		"a document accepted so far will be rejected"}},
	{[]string{"cvc-elt", "cvc-identity-constraint", "cvc-au", "cvc-complex-type", "cvc-attribute", "cvc-type"}, category{"G-assessment",
		"goxsdgo rejects a document by an assessment rule that libxml2 does not apply or applies differently",
		"XSD 1, cvc-* rules (3.3.4, 3.4.4, 3.5.4, 3.11.4)",
		"a document accepted so far will be rejected"}},
}

// Kategorie, gdy libxml2 odrzuca, a goxsdgo przyjmuje: według kodu libxml2.
func libRejects(kind w3c.Kind, code int) category {
	switch {
	case kind == w3c.SchemaTest:
		return category{"L-schema", "libxml2 rejects a schema that goxsdgo accepts",
			"depends on the case (list below)", "a schema rejected so far will be accepted"}
	case code >= 1824 && code <= 1840:
		return category{"L-values", "libxml2 rejects a simple-type value that goxsdgo accepts",
			"XSD 2, 4.1.4 and the facet rules", "a document rejected so far will be accepted"}
	case code < 1000 || code >= 2000:
		return category{"L-wellformed", "libxml2 finds a document not well-formed, goxsdgo does not",
			"XML 1.0, Namespaces 1.0", "a document rejected so far will be accepted"}
	}
	return category{"L-assessment", "libxml2 rejects a document by an assessment rule that goxsdgo applies differently",
		"XSD 1, cvc-* rules", "a document rejected so far will be accepted"}
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
	return "neither"
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
	fmt.Fprintf(&b, "# Verdict differences between goxsdgo and libxml2 on the W3C suite\n\n")
	fmt.Fprintf(&b, "Generated by `tools/compare` (`go run -tags libxml2 . -docs w3c`); do not edit by hand.\n")
	fmt.Fprintf(&b, "As of %s, libxml2 %s through `tools/compare`, W3C suite at commit `7bc3365c`. Categories and decisions:\n"+
		"[differences.md](differences.md) (in Polish); method: [methodology.md](methodology.md).\n\n",
		time.Now().Format("2006-01-02"), libxml2Version())
	fmt.Fprintf(&b, "Differences: %d (schema tests %d, instance tests %d); uncategorised: %d. \"Right\" means the verdict the W3C\n"+
		"suite expects.\n\n", len(all), len(schema), len(instance), len(uncategorized))
	b.WriteString("| Category | Cases | goxsdgo right | libxml2 right | Neither | Rule | Effect for a consumer moving from libxml2 |\n|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		a := byCat[k]
		fmt.Fprintf(&b, "| %s: %s | %d | %d | %d | %d | %s | %s |\n", k, a.cat.title, a.n, a.go_, a.lib, a.x, a.cat.rule, a.cat.effect)
	}
	if len(uncategorized) > 0 {
		b.WriteString("\n## Uncategorised\n\n")
		for _, d := range uncategorized {
			fmt.Fprintf(&b, "- `%s`: goxsdgo %s (%s), libxml2 %s (%d)\n", d.c.ID, d.goxsdgo, d.goCode, d.libxml2, d.libCode)
		}
	}
	b.WriteString("\n## Every case\n\n| Case | Status | Expected | goxsdgo | libxml2 | Category | Right |\n|---|---|---|---|---|---|---|\n")
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
