package main

// Przypadki sporne zestawu W3C: kryteria są obiektywne, ustalone z góry i te same dla wszystkich kandydatów
// (decyzja właściciela z 2026-11-25). Porównanie podaje wynik na wszystkich przypadkach w zakresie i na wspólnym
// zbiorze bez spornych, a docs/contested-cases.md wymienia każdy sporny przypadek z powodem.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/polestorm/goxsdgo/internal/w3c"
)

// contestRule to jedna reguła klasyfikacji. Reguły sprawdzamy po kolei; przypadek dostaje kategorię pierwszej
// pasującej reguły (nakładanie jest więc jawne: liczymy je osobno w testach i w dokumencie).
type contestRule struct {
	Category string // "a" … "d"
	Reason   string
	// contributor i groups wskazują grupy z nazwy; puste groups oznacza dopasowanie przez match.
	contributor string
	groups      []string
	instances   bool // tylko testy dokumentów
	match       func(c *w3c.Case) bool
}

// categoryTitles opisuje kategorie (kolejność = pierwszeństwo).
var categoryTitles = []struct{ ID, Title string }{
	{"a", "status queried, disputed-test or disputed-spec in the suite's metadata"},
	{"b", "the outcome depends on the edition of XML 1.0 (name character rules)"},
	{"c", "the outcome depends on the Unicode version"},
	{"d", "opposite expectations within the suite for the same situation (both sides of the contradiction)"},
	{"e", "an expectation against the unambiguous text of the 2e recommendation"},
}

var unicodeRegexGroups = []string{"reJ11", "reJ13", "reJ19", "reJ21", "reJ23", "reJ25", "reJ29", "reJ31", "reJ33",
	"reJ35", "reJ61", "reJ69", "reJ75", "reJ77", "reS21", "reS42", "reT63", "reT84"}

func contestRules() []contestRule {
	return []contestRule{
		{Category: "a", Reason: "status in the W3C suite's metadata", match: func(c *w3c.Case) bool {
			switch c.Status {
			case "queried", "disputed-test", "disputed-spec":
				return true
			}
			return false
		}},
		{Category: "b", Reason: "name characters by XML 1.0 2e (the edition XSD 1.0 2e refers to) or 5e (what the group " +
			"expects); goxsdgo has both, Config.NameRules, ADR 0012", match: func(c *w3c.Case) bool {
			return c.Contributor == "Saxonica" && c.Set == "XmlVersions"
		}},
		{Category: "b", Reason: "the class \\c by the CombiningChar production of XML 1.0 2e (a range ending at U+0345) " +
			"or of a later edition; ADR 0005, ADR 0012", contributor: "Microsoft", groups: []string{"reZ006i"}},
		{Category: "c", Reason: "character categories outside the BMP: the suite assumes Unicode 3.0 (no categories), " +
			"the Go standard library has Unicode 15; ADR 0005", contributor: "Microsoft", groups: unicodeRegexGroups,
			instances: true},
		{Category: "d", Reason: "an unresolved reference in an unused declaration: the Missing group expects a valid " +
			"schema (§5.3), while elemM002, xsd015.e, xsd016.e and idB005 expect an src-resolve error for the same " +
			"situation; ADR 0012", contributor: "Saxonica", groups: []string{"missing001", "missing002", "missing003", "missing006"}},
		{Category: "d", Reason: "the other side of the contradiction with the Missing group (an src-resolve error for an " +
			"unresolved reference in an unused declaration); ADR 0012", contributor: "Microsoft",
			groups: []string{"elemM002", "idB005"}},
		{Category: "d", Reason: "the other side of the contradiction with the Missing group; ADR 0012", contributor: "Sun",
			groups: []string{"xsd015.e", "xsd016.e"}},
		{Category: "d", Reason: "RecurseAsIfGroup (3.9.6): particlesZ001 needs the element's occurrence range carried over " +
			"to the group, particlesHa161 and particlesK006 the literal reading; ADR 0007", contributor: "Microsoft",
			groups: []string{"particlesZ001", "particlesHa161", "particlesK006"}},
	}
}

// contested to wynik klasyfikacji jednego przypadku.
type contested struct {
	Category string
	Reason   string
	// Also to pozostałe pasujące kategorie (nakładanie jest jawne; liczy się pierwsza).
	Also []string
}

// classify przypisuje sporne przypadki w zakresie do kategorii.
func classify(cases []*w3c.Case) map[string]contested {
	rules := contestRules()
	out := map[string]contested{}
	for _, c := range cases {
		if !c.InScope() {
			continue
		}
		for _, r := range rules {
			if !r.matches(c) {
				continue
			}
			k, ok := out[c.ID]
			if !ok {
				out[c.ID] = contested{Category: r.Category, Reason: r.Reason}
				continue
			}
			if r.Category != k.Category && !contains(k.Also, r.Category) {
				k.Also = append(k.Also, r.Category)
				out[c.ID] = k
			}
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (r contestRule) matches(c *w3c.Case) bool {
	if r.match != nil {
		return r.match(c)
	}
	if c.Contributor != r.contributor || (r.instances && c.Kind != w3c.InstanceTest) {
		return false
	}
	for _, g := range r.groups {
		if c.Group == g {
			return true
		}
	}
	return false
}

// verdicts to werdykty jednego kandydata z pliku TSV (id, expected, verdict).
type verdicts struct {
	Name string
	ByID map[string]string
}

// candidateFiles: kolejność kolumn w porównaniu i pliki wyników.
var candidateFiles = []struct{ Name, File string }{
	{"goxsdgo", "bench/w3c-goxsdgo.tsv"},
	{goxsdgo5Name, "bench/w3c-" + slug(goxsdgo5Name) + ".tsv"},
	{"libxml2 2.15.4", "compare/libxml2-w3c.tsv"},
	{"knroy/go-xml", "bench/w3c-knroy-go-xml.tsv"},
	{"jacoelho/xsd", "bench/w3c-jacoelho-xsd.tsv"},
	{"cB-Sigma/xsd", "bench/w3c-cb-sigma-xsd.tsv"},
}

func readVerdicts(name, path string) (*verdicts, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	v := &verdicts{Name: name, ByID: map[string]string{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		parts := strings.Split(sc.Text(), "\t")
		if len(parts) >= 3 {
			v.ByID[parts[0]] = parts[2]
		}
	}
	return v, sc.Err()
}

func (v *verdicts) passes(c *w3c.Case) bool {
	got := v.ByID[c.ID]
	return (got == "valid" || got == "invalid") && c.Matches(w3c.Validity(got))
}

// contestedScore to wynik kandydata: wszystkie, bez spornych, sporne.
type contestedScore struct {
	Name                           string
	All, Uncontested, Contested    int
	AllN, UncontestedN, ContestedN int
	ByCategory                     map[string]int
}

func runContested(root, out string, writeDocs bool) error {
	e, err := w3cEnv(root)
	if err != nil {
		return err
	}
	cases, err := w3c.Load(os.DirFS(e.dir), w3c.Tokens1_0)
	if err != nil {
		return err
	}
	cl := classify(cases)
	var vs []*verdicts
	for _, cf := range candidateFiles {
		v, err := readVerdicts(cf.Name, filepath.Join(root, "out", filepath.FromSlash(cf.File)))
		if err != nil {
			return fmt.Errorf("%s: %w (run w3c first; libxml2 from tools/compare w3c)", cf.Name, err)
		}
		vs = append(vs, v)
	}
	var inScope []*w3c.Case
	for _, c := range cases {
		if c.InScope() {
			inScope = append(inScope, c)
		}
	}
	scores := make([]contestedScore, len(vs))
	for i, v := range vs {
		s := contestedScore{Name: v.Name, ByCategory: map[string]int{}}
		for _, c := range inScope {
			p := v.passes(c)
			s.AllN++
			if p {
				s.All++
			}
			if k, ok := cl[c.ID]; ok {
				s.ContestedN++
				if p {
					s.Contested++
					s.ByCategory[k.Category]++
				}
			} else {
				s.UncontestedN++
				if p {
					s.Uncontested++
				}
			}
		}
		scores[i] = s
	}
	fmt.Printf("contested cases: %d of %d in scope\n", len(cl), len(inScope))
	for _, s := range scores {
		fmt.Printf("%-28s all %5d / %5d   uncontested %5d / %5d   contested %3d / %3d\n", s.Name, s.All, s.AllN,
			s.Uncontested, s.UncontestedN, s.Contested, s.ContestedN)
	}
	if err := writeJSON(out, "contested.json", scores); err != nil {
		return err
	}
	if writeDocs {
		return os.WriteFile(filepath.Join(root, "docs", "contested-cases.md"), []byte(contestedDoc(inScope, cl, vs, scores)), 0o644)
	}
	return nil
}

// rejected to przypadki rozważone i niewłączone, z powodem (część dokumentu, żeby wybór był sprawdzalny).
var rejected = []struct{ Cases, Reason string }{
	{"particlesZ035_a, particlesZ036_b1, particlesZ036_b2, particlesZ036_c (instances)",
		"goxsdgo stops on them with a limit error (`ErrLimit`, content models ambiguous in their counters, ADR 0011, " +
			"question 9): a limitation of goxsdgo, not a dispute about the expectation"},
	{"IDREFS_length006, NMTOKENS_length006 (schema tests, stable)",
		"on a second reading of 4.3.1.4 (length-minLength-maxLength, clause 1.2) the suite agrees with the text: the " +
			"built-in IDREFS and NMTOKENS already have minLength 1 without length, so minLength 1 next to length 5 is " +
			"allowed; this was a goxsdgo bug, fixed on 2026-11-25, not a dispute"},
	{"attP031 (schema and instance)",
		"the suite gives separate expectations for XSD 1.0 (valid) and 1.1 (invalid); the text of 3.2.2 on " +
			"`use=\"prohibited\"` is not unambiguous enough to set aside that decision of the working group"},
	{"anyURI_b006 (schema test)",
		"the suite gives separate expectations for XSD 1.0 (invalid) and 1.1 (valid); reading 3.2.17 through XLink " +
			"§5.4 is not unambiguous enough to set aside that decision of the working group"},
	{"the other cases goxsdgo fails",
		"they meet no criterion; that goxsdgo fails a case does not make it contested"},
}

func contestedDoc(inScope []*w3c.Case, cl map[string]contested, vs []*verdicts, scores []contestedScore) string {
	var b strings.Builder
	b.WriteString("# Contested cases of the W3C test suite\n\n")
	b.WriteString("Generated by `tools/bench` (`go -C tools/bench run . -root ../.. -docs contested`, after `w3c`);\n")
	b.WriteString("do not edit by hand. As of " + time.Now().Format("2006-01-02") +
		", W3C suite at commit `7bc3365c`, XSD 1.0 2e scope as in `TestW3CSuite`.\n\n")
	b.WriteString("The W3C XML Schema test suite is the common yardstick for XSD validators, but some of its cases do not\n")
	b.WriteString("have one right answer: the working group queried or disputed them, their outcome depends on which edition\n")
	b.WriteString("of XML or Unicode a validator follows, or the suite expects opposite verdicts for the same situation.\n")
	b.WriteString("Counting such cases rewards whoever happens to agree with one side, so the comparison in the README\n")
	b.WriteString("shows the score with and without them.\n\n")
	b.WriteString("The criteria were fixed in advance, before looking at who passes which case, and are the same for every\n")
	b.WriteString("candidate. They are objective: each one is a property of the suite (its metadata, its expectations) or of\n")
	b.WriteString("the standards it rests on, not of any validator's result. A case gets the category of the first\n")
	b.WriteString("criterion it meets; the other criteria it also meets are listed. In the contested cases goxsdgo follows\n")
	b.WriteString("the text of the recommendation; the cases it fails for that reason are explained one by one in\n")
	b.WriteString("[differences.md](differences.md) (in Polish). Cases that were considered and left out are listed below with the reason.\n\n## Criteria\n\n")
	count, also := map[string]int{}, map[string]int{}
	for _, k := range cl {
		count[k.Category]++
		for _, a := range k.Also {
			also[a]++
		}
	}
	b.WriteString("| Category | Criterion | Cases (first match) | Also met, counted earlier |\n|---|---|---|---|\n")
	for _, t := range categoryTitles {
		fmt.Fprintf(&b, "| %s | %s | %d | %d |\n", t.ID, t.Title, count[t.ID], also[t.ID])
	}
	fmt.Fprintf(&b, "| total | | %d of %d in scope | |\n\n", len(cl), len(inScope))
	b.WriteString("Category e is empty: none of the cases considered had an unambiguous quotation (below).\n\n")
	b.WriteString("## Results\n\n| Candidate | All in scope | Without contested | Contested passed |")
	for _, t := range categoryTitles {
		fmt.Fprintf(&b, " %s |", t.ID)
	}
	b.WriteString("\n|---|---|---|---|")
	for range categoryTitles {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, s := range scores {
		fmt.Fprintf(&b, "| %s | %d / %d | %d / %d | %d / %d |", s.Name, s.All, s.AllN, s.Uncontested, s.UncontestedN,
			s.Contested, s.ContestedN)
		for _, t := range categoryTitles {
			fmt.Fprintf(&b, " %d |", s.ByCategory[t.ID])
		}
		b.WriteString("\n")
	}
	b.WriteString("\nlibxml2 runs through `goxsdval` (`tools/compare`): cases that tool cannot run (a schema of several\n")
	b.WriteString("documents, a schema named only by hints) count as failed, as in the README table.\n\n")
	b.WriteString("## Considered and not included\n\n| Cases | Reason |\n|---|---|\n")
	for _, r := range rejected {
		fmt.Fprintf(&b, "| %s | %s |\n", r.Cases, r.Reason)
	}
	b.WriteString("\n## The contested cases\n\nCandidate columns: ✓ passed, · failed.\n\n| Case | Cat. | Expected |")
	for _, v := range vs {
		fmt.Fprintf(&b, " %s |", v.Name)
	}
	b.WriteString(" Reason |\n|---|---|---|")
	for range vs {
		b.WriteString("---|")
	}
	b.WriteString("---|\n")
	ids := make([]string, 0, len(cl))
	byID := map[string]*w3c.Case{}
	for _, c := range inScope {
		if _, ok := cl[c.ID]; ok {
			ids = append(ids, c.ID)
			byID[c.ID] = c
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		ci, cj := cl[ids[i]].Category, cl[ids[j]].Category
		if ci != cj {
			return ci < cj
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		c := byID[id]
		reason := cl[id].Reason
		if cl[id].Category == "a" {
			reason += ": " + c.Status
		}
		cat := cl[id].Category
		if len(cl[id].Also) > 0 {
			cat += " (also " + strings.Join(cl[id].Also, ", ") + ")"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |", id, cat, c.Expected)
		for _, v := range vs {
			if v.passes(c) {
				b.WriteString(" ✓ |")
			} else {
				b.WriteString(" · |")
			}
		}
		fmt.Fprintf(&b, " %s |\n", reason)
	}
	return b.String()
}
