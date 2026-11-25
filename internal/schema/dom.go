package schema

import (
	"errors"
	"io"
	"strings"

	"github.com/polestorm/goxsdgo/internal/xmlreader"
)

// node to element dokumentu schematu. Dokumenty schematu są małe, więc trzymamy je w pamięci w całości; walidacja
// dokumentów (etap 5) jest strumieniowa i drzewa nie buduje.
type node struct {
	space, local string
	attrs        []nodeAttr
	kids         []*node
	parent       *node
	// bindings to deklaracje przestrzeni nazw z tego elementu; razem z przodkami dają [in-scope namespaces].
	bindings []xmlreader.Binding
	// text: element ma tekst inny niż białe znaki (textLine, textCol: pierwszy taki tekst).
	text              bool
	textLine, textCol int
	line, col         int
	doc               *document
}

type nodeAttr struct {
	space, local, value string
	line, col           int
}

// is mówi, czy węzeł to element XSD o nazwie local.
func (n *node) is(local string) bool { return n.space == NamespaceXSD && n.local == local }

// attr zwraca atrybut bez przestrzeni nazw.
func (n *node) attr(local string) (string, bool) {
	for i := range n.attrs {
		if n.attrs[i].space == "" && n.attrs[i].local == local {
			return n.attrs[i].value, true
		}
	}
	return "", false
}

// has mówi, czy element ma atrybut bez przestrzeni nazw.
func (n *node) has(local string) bool {
	_, ok := n.attr(local)
	return ok
}

// xsdKids zwraca dzieci z przestrzeni XSD inne niż annotation.
func (n *node) xsdKids() []*node {
	var out []*node
	for _, k := range n.kids {
		if k.space == NamespaceXSD && k.local != "annotation" {
			out = append(out, k)
		}
	}
	return out
}

// kid zwraca pierwsze dziecko XSD o nazwie local albo nil.
func (n *node) kid(local string) *node {
	for _, k := range n.kids {
		if k.is(local) {
			return k
		}
	}
	return nil
}

// lookupPrefix rozwiązuje prefiks w [in-scope namespaces] elementu. Prefiks "xml" jest zawsze związany.
func (n *node) lookupPrefix(prefix string) (string, bool) {
	if prefix == "xml" {
		return NamespaceXML, true
	}
	for x := n; x != nil; x = x.parent {
		for i := len(x.bindings) - 1; i >= 0; i-- {
			if x.bindings[i].Prefix == prefix {
				if x.bindings[i].URI == "" && prefix != "" {
					return "", false
				}
				return x.bindings[i].URI, true
			}
		}
	}
	if prefix == "" {
		return "", true
	}
	return "", false
}

// LookupPrefix spełnia datatypes.NamespaceResolver dla wartości QName i NOTATION w dokumencie schematu.
func (n *node) LookupPrefix(prefix string) (string, bool) { return n.lookupPrefix(prefix) }

func (n *node) pos() Position {
	p := Position{Line: n.line, Column: n.col}
	if n.doc != nil {
		p.Document = n.doc.path
	}
	return p
}

func (n *node) attrPos(local string) Position {
	p := n.pos()
	for i := range n.attrs {
		if n.attrs[i].space == "" && n.attrs[i].local == local {
			p.Line, p.Column = n.attrs[i].line, n.attrs[i].col
		}
	}
	return p
}

// name zwraca nazwę elementu do komunikatów, np. "xs:element".
func (n *node) name() string {
	if n.space == NamespaceXSD {
		return "xs:" + n.local
	}
	if n.space == "" {
		return n.local
	}
	return "{" + n.space + "}" + n.local
}

// parseDocument czyta dokument schematu w całości do drzewa.
func parseDocument(r io.Reader, path string) (*node, error) {
	rd := xmlreader.New(r, xmlreader.Options{})
	var root, cur *node
	for {
		tok, err := rd.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			var xe *xmlreader.Error
			if errors.As(err, &xe) {
				code := CodeNotWellFormed
				if strings.HasPrefix(xe.Code, "unsupported-") {
					return nil, &Error{Code: xe.Code, Pos: Position{Document: path, Line: xe.Line, Column: xe.Column},
						Message: "schema document: " + xe.Message, Cause: errUnsupported}
				}
				return nil, &Error{Code: code, Pos: Position{Document: path, Line: xe.Line, Column: xe.Column},
					Message: "schema document is not well-formed: " + xe.Code + ": " + xe.Message, Cause: err}
			}
			return nil, &Error{Code: CodeNotWellFormed, Pos: Position{Document: path}, Message: err.Error(), Cause: err}
		}
		switch tok.Kind {
		case xmlreader.KindStartElement:
			n := &node{space: tok.Name.Space, local: tok.Name.Local, parent: cur, line: tok.Line, col: tok.Column,
				bindings: rd.DeclaredNamespaces()}
			for _, a := range tok.Attrs {
				n.attrs = append(n.attrs, nodeAttr{space: a.Name.Space, local: a.Name.Local, value: a.Value,
					line: a.Line, col: a.Column})
			}
			if cur == nil {
				root = n
			} else {
				cur.kids = append(cur.kids, n)
			}
			cur = n
		case xmlreader.KindEndElement:
			cur = cur.parent
		case xmlreader.KindText:
			if cur != nil && !tok.Whitespace && !cur.text {
				cur.text = true
				cur.textLine, cur.textCol = tok.Line, tok.Column
			}
		}
	}
	return root, nil
}
