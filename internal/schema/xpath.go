package schema

import (
	"strings"
)

// XPath to wyrażenie selector albo field ograniczenia tożsamości w podzbiorze z 3.11.6: alternatywa ścieżek.
type XPath struct {
	Source string
	Paths  []XPathPath
}

// XPathPath to ścieżka: opcjonalne „.//” na początku, kroki po osi child (albo „.”), a w polu także końcowy krok
// po osi attribute.
type XPathPath struct {
	Descendant bool
	Steps      []XPathStep
	// Attribute to końcowy test nazwy atrybutu (tylko field), albo nil.
	Attribute *NameTest
}

// XPathStep to krok: Self dla „.”, inaczej test nazwy elementu dziecka.
type XPathStep struct {
	Self bool
	Name NameTest
}

// NameTest to test nazwy: QName, „*” (AnyName) albo „p:*” (AnyLocal z przestrzenią Space). Nazwa bez prefiksu
// nie ma przestrzeni nazw (XPath 1.0 nie stosuje przestrzeni domyślnej).
type NameTest struct {
	AnyName, AnyLocal bool
	Space, Local      string
}

// xpathToken to token z [5] token: ".", "/", "//", "|", "@", "child::", "attribute::" albo NameTest.
type xpathToken struct {
	kind string // ".", "/", "//", "|", "@", "child", "attribute", "name"
	text string
}

func tokenizeXPath(s string) ([]xpathToken, string) {
	var out []xpathToken
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case strings.HasPrefix(s[i:], "//"):
			out = append(out, xpathToken{kind: "//"})
			i += 2
		case c == '/' || c == '|' || c == '@':
			out = append(out, xpathToken{kind: string(c)})
			i++
		case c == '.':
			out = append(out, xpathToken{kind: "."})
			i++
		case c == '*':
			out = append(out, xpathToken{kind: "name", text: "*"})
			i++
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t\n\r/|@*", rune(s[j])) && !(s[j] == ':' && j+1 < len(s) && s[j+1] == ':') {
				if s[j] == ':' && j+1 < len(s) && s[j+1] == '*' {
					j += 2
					break
				}
				j++
			}
			word := s[i:j]
			// Białe znaki są dozwolone między tokenami, także między nazwą osi a "::".
			k := j
			for k < len(s) && strings.ContainsRune(" \t\n\r", rune(s[k])) {
				k++
			}
			if strings.HasPrefix(s[k:], "::") {
				switch word {
				case "child":
					out = append(out, xpathToken{kind: "child"})
				case "attribute":
					out = append(out, xpathToken{kind: "attribute"})
				default:
					return nil, "axis " + word + ":: is not allowed"
				}
				i = k + 2
				continue
			}
			if word == "" {
				return nil, "unexpected character " + string(c)
			}
			out = append(out, xpathToken{kind: "name", text: word})
			i = j
		}
	}
	return out, ""
}

// parseXPath sprawdza wyrażenie według [1]–[7] z 3.11.6 i rozwiązuje prefiksy w kontekście n. Zwraca opis błędu
// albo "".
func parseXPath(src string, field bool, n *node, l *loader) (*XPath, string) {
	x := &XPath{Source: src}
	toks, msg := tokenizeXPath(src)
	if msg != "" {
		return x, msg
	}
	if len(toks) == 0 {
		return x, "empty expression"
	}
	pos := 0
	peek := func() string {
		if pos < len(toks) {
			return toks[pos].kind
		}
		return ""
	}
	nameTest := func() (NameTest, string) {
		if peek() != "name" {
			return NameTest{}, "name test expected"
		}
		t := toks[pos].text
		pos++
		if t == "*" {
			return NameTest{AnyName: true}, ""
		}
		prefix, local, has := strings.Cut(t, ":")
		if !has {
			if !validNCName(t) {
				return NameTest{}, t + " is not a valid name"
			}
			return NameTest{Local: t}, ""
		}
		if !validNCName(prefix) {
			return NameTest{}, t + " is not a valid name test"
		}
		ns, ok := n.lookupPrefix(prefix)
		if !ok || ns == "" {
			return NameTest{}, "prefix " + prefix + " is not declared"
		}
		if local == "*" {
			return NameTest{AnyLocal: true, Space: ns}, ""
		}
		if !validNCName(local) {
			return NameTest{}, t + " is not a valid name"
		}
		return NameTest{Space: ns, Local: local}, ""
	}
	for {
		var p XPathPath
		if peek() == "." && pos+1 < len(toks) && toks[pos+1].kind == "//" {
			p.Descendant = true
			pos += 2
		}
		for {
			switch peek() {
			case ".":
				pos++
				p.Steps = append(p.Steps, XPathStep{Self: true})
			case "child":
				pos++
				nt, msg := nameTest()
				if msg != "" {
					return x, msg
				}
				p.Steps = append(p.Steps, XPathStep{Name: nt})
			case "name":
				nt, msg := nameTest()
				if msg != "" {
					return x, msg
				}
				p.Steps = append(p.Steps, XPathStep{Name: nt})
			case "@", "attribute":
				if !field {
					return x, "a selector cannot select attributes"
				}
				pos++
				nt, msg := nameTest()
				if msg != "" {
					return x, msg
				}
				p.Attribute = &nt
				if k := peek(); k != "" && k != "|" {
					return x, "the attribute step must be the last step"
				}
			default:
				return x, "step expected"
			}
			if p.Attribute != nil || peek() != "/" {
				break
			}
			pos++ // "/"
		}
		x.Paths = append(x.Paths, p)
		switch peek() {
		case "":
			return x, ""
		case "|":
			pos++
		default:
			return x, "unexpected " + peek()
		}
	}
}
