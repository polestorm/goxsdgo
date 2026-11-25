// Package w3c czyta metadane zestawu testów W3C XML Schema (suite.xml i pliki *.testSet) i wybiera przypadki
// objęte zakresem goxsdgo.
//
// Format metadanych opisuje common/xsts.xsd w zestawie. Najważniejsze reguły:
//   - atrybut version na testSet, testGroup, schemaTest i instanceTest to lista żetonów połączonych „lub”:
//     przypadek jest objęty zakresem, gdy brak atrybutu albo wspieramy którykolwiek żeton;
//   - atrybut version na expected to lista żetonów połączonych „i”: oczekiwany wynik obowiązuje, gdy wspieramy
//     wszystkie żetony;
//   - przypadki dokumentów w grupie korzystają ze schematu z testu schematu tej samej grupy.
//
// Pakiet służy tylko testom i narzędziom; biblioteka go nie importuje.
package w3c

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// Tokens1_0 to żetony wersji, które wspiera goxsdgo: XSD 1.0, wydanie drugie (ADR 0001). Żetony wersji Unicode
// (Unicode_4.0.0, Unicode_6.0.0) celowo pomijamy do decyzji w pytaniu 7, więc przypadki zależne od wersji Unicode
// wypadają jako wyłączone z powodem.
var Tokens1_0 = []string{"1.0", "1.0-2e"}

// Kind odróżnia test schematu od testu dokumentu.
type Kind string

const (
	SchemaTest   Kind = "schema"
	InstanceTest Kind = "instance"
)

// Validity to oczekiwany albo uzyskany werdykt.
type Validity string

const (
	Valid   Validity = "valid"
	Invalid Validity = "invalid"
	// NotKnown dotyczy dokumentów: walidator dokumentów nie musi odróżniać go od Invalid (xsts.xsd).
	NotKnown Validity = "notKnown"
)

// Case to jeden przypadek testowy.
type Case struct {
	// ID jest stały i jednoznaczny w zestawie: "<testSet>/<testGroup>/<test>".
	ID string
	// Contributor to dostawca według katalogu metadanych (NIST, Microsoft, Sun, Boeing, ...).
	Contributor string
	Set, Group  string
	Kind        Kind
	// SchemaDocs to dokumenty schematu grupy (ścieżki w fs zestawu). Dla testu dokumentu bywa puste: wtedy
	// dokument wskazuje schemat przez xsi:schemaLocation.
	SchemaDocs []string
	// Instance to dokument do walidacji (tylko dla InstanceTest).
	Instance string
	// Expected to oczekiwany werdykt; puste, gdy przypadek jest wyłączony.
	Expected Validity
	// Status to bieżący status przypadku w zestawie (accepted, stable, queried, disputed-test, ...).
	Status string
	// Excluded podaje powód wyłączenia z zakresu; puste dla przypadków objętych zakresem.
	Excluded string
}

// InScope mówi, czy przypadek liczy się do kryterium K1.
func (c *Case) InScope() bool { return c.Excluded == "" }

// Matches mówi, czy uzyskany werdykt zgadza się z oczekiwanym.
func (c *Case) Matches(got Validity) bool {
	if c.Expected == got {
		return true
	}
	// Dokument bez znanej poprawności: wystarczy, że walidator go nie przyjmie.
	return c.Kind == InstanceTest && c.Expected == NotKnown && got == Invalid
}

// Load czyta suite.xml z fsys i wszystkie wskazane w nim pliki testSet. Przypadki spoza zakresu dla żetonów
// tokens są zwracane z ustawionym polem Excluded, żeby raport mógł je policzyć.
func Load(fsys fs.FS, tokens []string) ([]*Case, error) {
	supported := map[string]bool{}
	for _, t := range tokens {
		supported[t] = true
	}
	var suite struct {
		Version string `xml:"version,attr"`
		Refs    []struct {
			Href string `xml:"http://www.w3.org/1999/xlink href,attr"`
		} `xml:"testSetRef"`
	}
	if err := decode(fsys, "suite.xml", &suite); err != nil {
		return nil, err
	}
	var cases []*Case
	seen := map[string]bool{}
	for _, ref := range suite.Refs {
		p := path.Clean(ref.Href)
		var set testSet
		if err := decode(fsys, p, &set); err != nil {
			return nil, err
		}
		dir := path.Dir(p)
		contributor := contributorOf(p)
		setReason := excludedBy(set.Version, supported)
		for _, g := range set.Groups {
			groupReason := first(setReason, excludedBy(g.Version, supported))
			var docs []string
			if g.Schema != nil {
				for _, d := range g.Schema.Docs {
					q, err := resolve(dir, d.Href)
					if err != nil {
						return nil, fmt.Errorf("%s: group %s: %w", p, g.Name, err)
					}
					docs = append(docs, q)
				}
			}
			add := func(kind Kind, name, version string, instance string, expected []expected, current *status) error {
				c := &Case{
					ID:          set.Name + "/" + g.Name + "/" + name,
					Contributor: contributor,
					Set:         set.Name,
					Group:       g.Name,
					Kind:        kind,
					SchemaDocs:  docs,
					Instance:    instance,
				}
				if current != nil {
					c.Status = current.Status
				}
				if seen[c.ID] {
					return fmt.Errorf("%s: duplicate test %s", p, c.ID)
				}
				seen[c.ID] = true
				c.Excluded = first(groupReason, excludedBy(version, supported))
				if c.Excluded == "" {
					c.Excluded = missing(fsys, append(append([]string{}, docs...), instance))
				}
				if c.Excluded == "" {
					c.Expected, c.Excluded = choose(expected, supported)
				}
				cases = append(cases, c)
				return nil
			}
			if g.Schema != nil {
				if err := add(SchemaTest, g.Schema.Name, g.Schema.Version, "", g.Schema.Expected, g.Schema.Current); err != nil {
					return nil, err
				}
			}
			for _, it := range g.Instances {
				inst, err := resolve(dir, it.Doc.Href)
				if err != nil {
					return nil, fmt.Errorf("%s: group %s: %w", p, g.Name, err)
				}
				if err := add(InstanceTest, it.Name, it.Version, inst, it.Expected, it.Current); err != nil {
					return nil, err
				}
			}
		}
	}
	return cases, nil
}

type testSet struct {
	Name    string `xml:"name,attr"`
	Version string `xml:"version,attr"`
	Groups  []struct {
		Name    string `xml:"name,attr"`
		Version string `xml:"version,attr"`
		Schema  *struct {
			Name     string     `xml:"name,attr"`
			Version  string     `xml:"version,attr"`
			Docs     []href     `xml:"schemaDocument"`
			Expected []expected `xml:"expected"`
			Current  *status    `xml:"current"`
		} `xml:"schemaTest"`
		Instances []struct {
			Name     string     `xml:"name,attr"`
			Version  string     `xml:"version,attr"`
			Doc      href       `xml:"instanceDocument"`
			Expected []expected `xml:"expected"`
			Current  *status    `xml:"current"`
		} `xml:"instanceTest"`
	} `xml:"testGroup"`
}

type href struct {
	Href string `xml:"http://www.w3.org/1999/xlink href,attr"`
}

type expected struct {
	Validity string `xml:"validity,attr"`
	Version  string `xml:"version,attr"`
}

type status struct {
	Status string `xml:"status,attr"`
}

func decode(fsys fs.FS, name string, v any) error {
	f, err := fsys.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := xml.NewDecoder(f).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// resolve zamienia odnośnik względem pliku metadanych na ścieżkę w fs zestawu; odrzuca wyjście poza zestaw.
func resolve(dir, href string) (string, error) {
	if strings.Contains(href, ":") {
		return "", fmt.Errorf("absolute reference %q", href)
	}
	p := path.Join(dir, href)
	if !fs.ValidPath(p) {
		return "", fmt.Errorf("reference %q leaves the suite", href)
	}
	return p, nil
}

// excludedBy zwraca powód wyłączenia, gdy atrybut version kontenera nie zawiera żadnego wspieranego żetonu.
func excludedBy(version string, supported map[string]bool) string {
	tokens := strings.Fields(version)
	if len(tokens) == 0 {
		return ""
	}
	for _, t := range tokens {
		if supported[t] {
			return ""
		}
	}
	return "version " + strings.Join(tokens, " ")
}

// choose wybiera oczekiwany werdykt obowiązujący dla wspieranych żetonów.
func choose(list []expected, supported map[string]bool) (Validity, string) {
	var found []string
	for _, e := range list {
		ok := true
		for _, t := range strings.Fields(e.Version) {
			if !supported[t] {
				ok = false
			}
		}
		if ok {
			found = append(found, e.Validity)
		}
	}
	switch {
	case len(found) == 0:
		return "", "no expected outcome for XSD 1.0 2e"
	case len(found) > 1:
		return "", "several expected outcomes: " + strings.Join(found, ", ")
	}
	switch v := Validity(found[0]); v {
	case Valid, Invalid, NotKnown:
		return v, ""
	default:
		return "", "expected outcome " + found[0]
	}
}

// missing zwraca powód wyłączenia, gdy zestaw nie zawiera wskazanego pliku (usterka zestawu, np. wgData/sg).
func missing(fsys fs.FS, paths []string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := fs.Stat(fsys, p); err != nil {
			return "file missing from the suite: " + p
		}
	}
	return ""
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// contributorOf ustala dostawcę z katalogu metadanych; atrybut contributor bywa opisem, nie nazwą.
func contributorOf(p string) string {
	dir, _, _ := strings.Cut(p, "/")
	switch dir {
	case "nistMeta":
		return "NIST"
	case "msMeta":
		return "Microsoft"
	case "sunMeta":
		return "Sun"
	case "boeingMeta":
		return "Boeing"
	case "saxonMeta":
		return "Saxonica"
	case "ibmMeta":
		return "IBM"
	case "oracleMeta":
		return "Oracle"
	case "wgMeta":
		return "W3C WG"
	case "common":
		return "W3C"
	}
	return dir
}
