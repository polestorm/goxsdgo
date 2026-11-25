package xmlreader

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxsdgo/internal/testkit"
)

var update = flag.Bool("update", false, "rewrite the ratchet file testdata/w3c/xmlconf.ratchet")

const (
	xmlconfRoot    = "../../testdata/w3c/xmlconf"
	xmlconfRatchet = "../../testdata/w3c/xmlconf.ratchet"
)

// confTest to jeden przypadek zestawu W3C XML Conformance (element TEST, testcases.dtd).
type confTest struct {
	Group          string
	ID             string `xml:"ID,attr"`
	Type           string `xml:"TYPE,attr"`
	Entities       string `xml:"ENTITIES,attr"`
	URI            string `xml:"URI,attr"`
	Output         string `xml:"OUTPUT,attr"`
	Recommendation string `xml:"RECOMMENDATION,attr"`
	Version        string `xml:"VERSION,attr"`
	Edition        string `xml:"EDITION,attr"`
	Namespace      string `xml:"NAMESPACE,attr"`
	Sections       string `xml:"SECTIONS,attr"`
	dir            string
	excluded       string
}

var entityDecl = regexp.MustCompile(`<!ENTITY\s+(\S+)\s+SYSTEM\s+"([^"]+)"\s*>`)

// loadConformance czyta xmlconf.xml i pliki przypadków wskazane w nim przez encje zewnętrzne. Do metadanych
// zestawu (dane testowe, zaufane) wystarcza encoding/xml.
func loadConformance(t *testing.T) []*confTest {
	b, err := os.ReadFile(path.Join(xmlconfRoot, "xmlconf.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var tests []*confTest
	seen := map[string]bool{}
	for _, m := range entityDecl.FindAllSubmatch(b, -1) {
		file := string(m[2])
		data, err := os.ReadFile(path.Join(xmlconfRoot, file))
		if err != nil {
			t.Fatal(err)
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		dec.Strict = false
		dec.Entity = xml.HTMLEntity
		bases := []string{path.Dir(file)}
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			switch e := tok.(type) {
			case xml.StartElement:
				base := bases[len(bases)-1]
				for _, a := range e.Attr {
					if a.Name.Local == "base" {
						base = path.Join(base, a.Value)
					}
				}
				bases = append(bases, base)
				if e.Name.Local == "TEST" {
					ct := &confTest{Group: strings.SplitN(file, "/", 2)[0], Entities: "none", Recommendation: "XML1.0", Namespace: "yes"}
					for _, a := range e.Attr {
						switch a.Name.Local {
						case "ID":
							ct.ID = a.Value
						case "TYPE":
							ct.Type = a.Value
						case "ENTITIES":
							ct.Entities = a.Value
						case "URI":
							ct.URI = a.Value
						case "OUTPUT":
							ct.Output = a.Value
						case "RECOMMENDATION":
							ct.Recommendation = a.Value
						case "VERSION":
							ct.Version = a.Value
						case "EDITION":
							ct.Edition = a.Value
						case "NAMESPACE":
							ct.Namespace = a.Value
						case "SECTIONS":
							ct.Sections = a.Value
						}
					}
					ct.dir = base
					key := ct.Group + "/" + ct.ID
					if seen[key] {
						ct.ID = ct.ID + "@" + ct.URI
					}
					seen[ct.Group+"/"+ct.ID] = true
					ct.excluded = exclusion(ct)
					tests = append(tests, ct)
				}
			case xml.EndElement:
				bases = bases[:len(bases)-1]
			}
		}
	}
	return tests
}

var declaredEncoding = regexp.MustCompile(`^(?:\xef\xbb\xbf)?<\?xml[^>]*encoding\s*=\s*["']([A-Za-z][A-Za-z0-9._-]*)["']`)

// exclusion podaje powód wyłączenia przypadku z zakresu (ADR 0004) albo "".
func exclusion(ct *confTest) string {
	fields := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, f := range strings.Fields(s) {
			m[f] = true
		}
		return m
	}
	switch {
	case ct.Recommendation == "XML1.1" || ct.Recommendation == "NS1.1":
		return "XML 1.1 or Namespaces 1.1"
	case ct.Version != "" && !fields(ct.Version)["1.0"]:
		return "XML 1.1"
	case ct.Edition != "" && !fields(ct.Edition)["5"]:
		return "applies only to XML 1.0 editions 1-4"
	case ct.Entities != "none":
		return "needs external entities, which are never read"
	case ct.Type == "error":
		return "optional error (processors need not report it)"
	case ct.Namespace == "no" && strings.HasPrefix(ct.Recommendation, "NS"):
		return "namespace processing off"
	}
	if ct.Type == "valid" || ct.Type == "invalid" {
		if data, err := os.ReadFile(path.Join(xmlconfRoot, ct.dir, ct.URI)); err == nil {
			if m := declaredEncoding.FindSubmatch(data); m != nil && !supportedEncoding(string(m[1])) {
				return "encoding " + strings.ToLower(string(m[1])) + " not supported"
			}
		}
	}
	return ""
}

func supportedEncoding(name string) bool {
	switch strings.ToLower(name) {
	case "utf-8", "utf-16", "utf-16le", "utf-16be", "iso-8859-1", "iso_8859-1", "latin1", "l1", "iso-ir-100", "cp819",
		"ibm819", "csisolatin1", "us-ascii", "ascii", "iso646-us", "ansi_x3.4-1968", "csascii":
		return true
	}
	return false
}

// TestXMLConformance mierzy czytnik na zestawie W3C XML Conformance: dokumenty "valid" i "invalid" (walidacja DTD
// nie jest w zakresie) muszą zostać przeczytane bez błędu, a gdy jest OUTPUT, dać tę samą drugą postać
// kanoniczną; dokumenty "not-wf" muszą zostać odrzucone. Przypadki XML 1.0 są czytane bez przestrzeni nazw,
// przypadki Namespaces 1.0 z przestrzeniami nazw.
// REQ: XML-001 XML-002 XML-003 XML-004 XML-005 XML-006 XML-007 XML-008 XML-009 XML-010 XML-011 XML-012 XML-013
func TestXMLConformance(t *testing.T) {
	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err != nil {
		t.Skip(testkit.FetchHint)
	}
	tests := loadConformance(t)
	passed := map[string]bool{}
	inScope := map[string]bool{}
	type row struct{ in, ok int }
	rows := map[string]*row{}
	excluded := map[string]int{}
	var failures []string
	for _, ct := range tests {
		if ct.excluded != "" {
			excluded[ct.excluded]++
			continue
		}
		rw := rows[ct.Group]
		if rw == nil {
			rw = &row{}
			rows[ct.Group] = rw
		}
		rw.in++
		inScope[ct.Group+"/"+ct.ID] = true
		ok, why := runConformance(ct)
		if ok {
			rw.ok++
			passed[ct.Group+"/"+ct.ID] = true
		} else {
			failures = append(failures, fmt.Sprintf("%s/%s (%s %s, %s): %s", ct.Group, ct.ID, ct.Type, ct.URI, ct.Sections, why))
		}
	}
	var b strings.Builder
	groups := make([]string, 0, len(rows))
	for g := range rows {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	total := row{}
	fmt.Fprintf(&b, "W3C XML Conformance: passed/in scope\n")
	for _, g := range groups {
		fmt.Fprintf(&b, "%-10s %5d/%-5d\n", g, rows[g].ok, rows[g].in)
		total.in += rows[g].in
		total.ok += rows[g].ok
	}
	fmt.Fprintf(&b, "%-10s %5d/%-5d\n", "all", total.ok, total.in)
	reasons := make([]string, 0, len(excluded))
	for r := range excluded {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		fmt.Fprintf(&b, "excluded %5d: %s\n", excluded[r], r)
	}
	t.Log("\n" + b.String())
	if testing.Verbose() {
		for _, f := range failures {
			t.Log("fail: " + f)
		}
	}

	testkit.Ratchet(t, xmlconfRatchet, "W3C XML Conformance (TestXMLConformance, go test ./internal/xmlreader -update).",
		inScope, passed, *update)
}

func runConformance(ct *confTest) (bool, string) {
	file := path.Join(xmlconfRoot, ct.dir, ct.URI)
	f, err := os.Open(file)
	if err != nil {
		return false, err.Error()
	}
	defer f.Close()
	opts := Options{NoNamespaces: !strings.HasPrefix(ct.Recommendation, "NS")}
	canon, err := canonical(New(f, opts))
	if ct.Type == "not-wf" {
		if err == nil {
			return false, "accepted"
		}
		return true, ""
	}
	if err != nil {
		return false, err.Error()
	}
	if ct.Output != "" {
		want, err := os.ReadFile(path.Join(xmlconfRoot, ct.dir, ct.Output))
		if err != nil {
			return false, err.Error()
		}
		if !bytes.Equal(canon, want) {
			return false, fmt.Sprintf("canonical form differs:\n got  %q\n want %q", canon, want)
		}
	}
	return true, ""
}

// canonical czyta cały dokument i zwraca jego drugą postać kanoniczną (James Clark, z deklaracjami notacji;
// opis w testcases.dtd zestawu).
func canonical(r *Reader) ([]byte, error) {
	var out bytes.Buffer
	var root string
	escape := func(s []byte) {
		for _, c := range string(s) {
			switch c {
			case '&':
				out.WriteString("&amp;")
			case '<':
				out.WriteString("&lt;")
			case '>':
				out.WriteString("&gt;")
			case '"':
				out.WriteString("&quot;")
			case '\t':
				out.WriteString("&#9;")
			case '\n':
				out.WriteString("&#10;")
			case '\r':
				out.WriteString("&#13;")
			default:
				out.WriteRune(c)
			}
		}
	}
	name := func(n Name) string {
		if n.Prefix != "" {
			return n.Prefix + ":" + n.Local
		}
		return n.Local
	}
	wroteDoctype := false
	for {
		t, err := r.Next()
		if err == io.EOF {
			return out.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
		switch t.Kind {
		case KindDoctype:
			root = t.Name.Local
			if !wroteDoctype {
				wroteDoctype = true
				if n := r.Notations(); len(n) > 0 {
					sorted := append([]Notation(nil), n...)
					sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
					out.WriteString("<!DOCTYPE " + root + " [\n")
					for _, x := range sorted {
						out.WriteString("<!NOTATION " + x.Name)
						switch {
						case x.HasPublic && x.HasSystem:
							out.WriteString(" PUBLIC '" + x.PublicID + "' '" + x.SystemID + "'")
						case x.HasPublic:
							out.WriteString(" PUBLIC '" + x.PublicID + "'")
						default:
							out.WriteString(" SYSTEM '" + x.SystemID + "'")
						}
						out.WriteString(">\n")
					}
					out.WriteString("]>\n")
				}
			}
		case KindStartElement:
			out.WriteString("<" + name(t.Name))
			attrs := append([]Attr(nil), t.Attrs...)
			sort.Slice(attrs, func(i, j int) bool { return name(attrs[i].Name) < name(attrs[j].Name) })
			for _, a := range attrs {
				out.WriteString(" " + name(a.Name) + `="`)
				escape([]byte(a.Value))
				out.WriteString(`"`)
			}
			out.WriteString(">")
		case KindEndElement:
			out.WriteString("</" + name(t.Name) + ">")
		case KindText:
			escape(t.Data)
		case KindProcInst:
			out.WriteString("<?" + t.Name.Local + " ")
			out.Write(t.Data)
			out.WriteString("?>")
		}
	}
}
