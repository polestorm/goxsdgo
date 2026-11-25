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
	{"a", "status queried, disputed-test albo disputed-spec w metadanych zestawu"},
	{"b", "wynik zależy od wydania XML 1.0 (reguły znaków nazw)"},
	{"c", "wynik zależy od wersji Unicode"},
	{"d", "sprzeczne oczekiwania wewnątrz zestawu dla tej samej sytuacji (oba końce sprzeczności)"},
	{"e", "oczekiwanie sprzeczne z jednoznacznym tekstem rekomendacji 2e"},
}

var unicodeRegexGroups = []string{"reJ11", "reJ13", "reJ19", "reJ21", "reJ23", "reJ25", "reJ29", "reJ31", "reJ33",
	"reJ35", "reJ61", "reJ69", "reJ75", "reJ77", "reS21", "reS42", "reT63", "reT84"}

func contestRules() []contestRule {
	return []contestRule{
		{Category: "a", Reason: "status w metadanych zestawu W3C", match: func(c *w3c.Case) bool {
			switch c.Status {
			case "queried", "disputed-test", "disputed-spec":
				return true
			}
			return false
		}},
		{Category: "b", Reason: "znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); " +
			"goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012", match: func(c *w3c.Case) bool {
			return c.Contributor == "Saxonica" && c.Set == "XmlVersions"
		}},
		{Category: "b", Reason: "klasa \\c według produkcji CombiningChar XML 1.0 2e (koniec przedziału na U+0345) " +
			"albo nowszego wydania; ADR 0005, ADR 0012", contributor: "Microsoft", groups: []string{"reZ006i"}},
		{Category: "c", Reason: "kategorie znaków spoza BMP: zestaw zakłada Unicode 3.0 (bez kategorii), " +
			"biblioteka standardowa Go ma Unicode 15; ADR 0005", contributor: "Microsoft", groups: unicodeRegexGroups,
			instances: true},
		{Category: "d", Reason: "nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu " +
			"poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; " +
			"ADR 0012", contributor: "Saxonica", groups: []string{"missing001", "missing002", "missing003", "missing006"}},
		{Category: "d", Reason: "przeciwny koniec sprzeczności z grupą Missing (błąd src-resolve dla nierozwiązanego " +
			"odwołania w nieużywanej deklaracji); ADR 0012", contributor: "Microsoft",
			groups: []string{"elemM002", "idB005"}},
		{Category: "d", Reason: "przeciwny koniec sprzeczności z grupą Missing; ADR 0012", contributor: "Sun",
			groups: []string{"xsd015.e", "xsd016.e"}},
		{Category: "d", Reason: "RecurseAsIfGroup (3.9.6): particlesZ001 wymaga przeniesienia liczności elementu na " +
			"grupę, particlesHa161 i particlesK006 — odczytu dosłownego; ADR 0007", contributor: "Microsoft",
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
	{"particlesZ035_a, particlesZ036_b1, particlesZ036_b2, particlesZ036_c (dokumenty)",
		"goxsdgo kończy je błędem limitu (`ErrLimit`, modele niejednoznaczne co do liczników, ADR 0011, pytanie 9): " +
			"to ograniczenie goxsdgo, nie spór o oczekiwanie"},
	{"IDREFS_length006, NMTOKENS_length006 (testy schematów, stable)",
		"po ponownym przeczytaniu 4.3.1.4 (length-minLength-maxLength, punkt 1.2) oczekiwanie zestawu jest zgodne " +
			"z tekstem: typy wbudowane IDREFS i NMTOKENS mają już minLength 1 bez length, więc minLength 1 obok " +
			"length 5 jest dozwolone; to błąd goxsdgo do poprawienia, nie spór"},
	{"attP031 (schemat i dokument)",
		"zestaw rozdziela oczekiwania dla XSD 1.0 (poprawny) i 1.1 (błędny); tekst 3.2.2 o `use=\"prohibited\"` nie jest " +
			"tu jednoznaczny wobec tej decyzji grupy roboczej"},
	{"anyURI_b006 (test schematu)",
		"zestaw rozdziela oczekiwania dla XSD 1.0 (błędny) i 1.1 (poprawny); odczyt 3.2.17 przez XLink §5.4 nie jest " +
			"tu jednoznaczny wobec tej decyzji grupy roboczej"},
	{"pozostałe przypadki, które goxsdgo oblewa",
		"nie spełniają żadnego kryterium; samo to, że goxsdgo je oblewa, nie czyni ich spornymi"},
}

func contestedDoc(inScope []*w3c.Case, cl map[string]contested, vs []*verdicts, scores []contestedScore) string {
	var b strings.Builder
	b.WriteString("# Przypadki sporne zestawu W3C w porównaniu z innymi walidatorami\n\n")
	b.WriteString("Plik generowany przez `tools/bench` (`go -C tools/bench run . -root ../.. -docs contested`, po `w3c`);\n")
	b.WriteString("nie edytuj go ręcznie. Stan: " + time.Now().Format("2006-01-02") +
		", zestaw W3C z commitu `7bc3365c`, zakres XSD 1.0 2e jak `TestW3CSuite`.\n\n")
	b.WriteString("Decyzja właściciela z 2026-11-25: w porównaniu z innymi walidatorami przypadki sporne są wyłączane albo\n")
	b.WriteString("opatrzone zastrzeżeniem. Kryteria są ustalone z góry, obiektywne i te same dla wszystkich kandydatów;\n")
	b.WriteString("przypadek dostaje kategorię pierwszego pasującego kryterium. W przypadkach spornych goxsdgo idzie za\n")
	b.WriteString("tekstem rekomendacji (`differences.md`).\n\n## Kryteria\n\n")
	count, also := map[string]int{}, map[string]int{}
	for _, k := range cl {
		count[k.Category]++
		for _, a := range k.Also {
			also[a]++
		}
	}
	b.WriteString("| Kategoria | Kryterium | Przypadki (pierwsza pasująca) | Pasuje też, liczone wcześniej |\n|---|---|---|---|\n")
	for _, t := range categoryTitles {
		fmt.Fprintf(&b, "| %s | %s | %d | %d |\n", t.ID, t.Title, count[t.ID], also[t.ID])
	}
	fmt.Fprintf(&b, "| razem | | %d z %d w zakresie | |\n\n", len(cl), len(inScope))
	b.WriteString("Kategoria e jest pusta: żaden rozważony przypadek nie miał jednoznacznego cytatu (niżej).\n\n")
	b.WriteString("## Wyniki\n\n| Kandydat | Wszystkie w zakresie | Bez spornych | Sporne zaliczone |")
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
	b.WriteString("\nlibxml2 przez `goxsdval` (`tools/compare`): przypadki, których to narzędzie nie umie uruchomić (schemat z kilku\n")
	b.WriteString("dokumentów, schemat tylko ze wskazówek), liczą się jako niezaliczone, jak w tabeli w README.\n\n")
	b.WriteString("## Rozważone i niewłączone\n\n| Przypadki | Powód |\n|---|---|\n")
	for _, r := range rejected {
		fmt.Fprintf(&b, "| %s | %s |\n", r.Cases, r.Reason)
	}
	b.WriteString("\n## Lista przypadków spornych\n\nKolumny kandydatów: ✓ zaliczony, · niezaliczony.\n\n| Przypadek | Kat. | Oczekiwane |")
	for _, v := range vs {
		fmt.Fprintf(&b, " %s |", v.Name)
	}
	b.WriteString(" Powód |\n|---|---|---|")
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
			cat += " (też " + strings.Join(cl[id].Also, ", ") + ")"
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
