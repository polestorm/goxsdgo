package schema

import "strings"

// qname interpretuje wartość QName v w kontekście elementu n (3.15.3, QName Interpretation). W dokumencie
// włączonym jako kameleon brak przestrzeni zamieniamy na docelową przestrzeń włączającego (4.2.1, punkt 3.2).
func (l *loader) qname(n *node, v string) (QName, bool) {
	v = collapse(v)
	prefix, local, has := strings.Cut(v, ":")
	if !has {
		prefix, local = "", v
	}
	ns, ok := n.lookupPrefix(prefix)
	if !ok {
		return QName{}, false
	}
	if ns == "" && !n.doc.hasTNS && n.doc.tns != "" {
		ns = n.doc.tns
	}
	return QName{Space: ns, Local: local}, true
}

// nsAllowed sprawdza punkt 4 z QName resolution (Schema Document): przestrzeń odwołania musi być docelową
// przestrzenią dokumentu albo zaimportowaną. Przestrzeń XSD jest zawsze dostępna (typy wbudowane), ADR 0006.
func nsAllowed(d *document, ns string) bool {
	if ns == NamespaceXSD {
		return true
	}
	if ns == "" {
		return !d.hasTNS && d.tns == "" || d.importsNoNS
	}
	return ns == d.tns || d.imports[ns]
}

// topDef zwraca def elementu najwyższego poziomu (dziecka schema albo redefine), w którym leży n.
func (l *loader) topDef(n *node) *def {
	for x := n; x != nil; x = x.parent {
		if x.parent != nil && (x.parent.is("schema") || x.parent.is("redefine")) {
			return l.defOf[x]
		}
	}
	return nil
}

// resolve rozwiązuje wartość v atrybutu attr elementu n na komponent rodzaju kind (3.15.3). Odwołanie
// redefiniowanej grupy albo grupy atrybutów do siebie wskazuje oryginał (4.2.2, Individual Component
// Redefinition 2). Błąd zgłasza samo; zwraca nil, gdy nie da się rozwiązać.
func (l *loader) resolve(n *node, attr, v string, kind defKind) *def {
	q, ok := l.resolveName(n, attr, v)
	if !ok {
		return nil
	}
	if kind == kindGroup || kind == kindAttributeGroup {
		if self := l.topDef(n); self != nil && self.orig != nil && self.kind == kind && self.name == q {
			return self.orig
		}
	}
	df := l.index[kind][q]
	if df == nil && kind == kindAttribute && (q.Space == NamespaceXML || q.Space == NamespaceXSI) {
		initBuiltins()
		decl := xmlAttributes[q.Local]
		if q.Space == NamespaceXSI {
			decl = xsiAttributes[q.Local]
		}
		if decl != nil {
			return &def{kind: kindAttribute, name: q, comp: decl, state: stateBuilt}
		}
	}
	if df == nil {
		l.errorf(CodeResolve, n.attrPos(attr), "%s %s referenced by %s/@%s is not defined", kindNames[kind], q, n.name(), attr)
		return nil
	}
	return df
}

// resolveName interpretuje QName i sprawdza dostępność przestrzeni; błędy zgłasza samo.
func (l *loader) resolveName(n *node, attr, v string) (QName, bool) {
	q, ok := l.qname(n, v)
	if !ok {
		l.errorf(CodeQName, n.attrPos(attr), "prefix of %q in %s/@%s is not declared", collapse(v), n.name(), attr)
		return QName{}, false
	}
	if !nsAllowed(n.doc, q.Space) {
		clause := ".4.2"
		if q.Space == "" {
			clause = ".4.1"
		}
		l.errorf(CodeResolve+clause, n.attrPos(attr), "%s/@%s refers to namespace %q, which the schema document neither targets nor imports",
			n.name(), attr, q.Space)
		return QName{}, false
	}
	return q, true
}

// typeDef rozwiązuje odwołanie do definicji typu. Zwraca typ (wbudowany albo z indeksu) i jego def (nil dla
// wbudowanych). Gdy base jest prawdą, odwołanie redefiniowanego typu do jego własnej nazwy wskazuje oryginał.
func (l *loader) typeDef(n *node, attr, v string, base bool) (Type, *def) {
	q, ok := l.resolveName(n, attr, v)
	if !ok {
		return nil, nil
	}
	if q.Space == NamespaceXSD {
		initBuiltins()
		if q.Local == "anyType" {
			return anyType, nil
		}
		if st := builtinTypes[q.Local]; st != nil {
			return st, nil
		}
	}
	if base {
		if self := l.topDef(n); self != nil && self.orig != nil && self.kind == kindType && self.name == q {
			return self.orig.comp.(Type), self.orig
		}
	}
	df := l.index[kindType][q]
	if df == nil {
		l.errorf(CodeResolve, n.attrPos(attr), "type definition %s referenced by %s/@%s is not defined", q, n.name(), attr)
		return nil, nil
	}
	return df.comp.(Type), df
}

// ensuredType rozwiązuje odwołanie do typu i buduje jego definicję, gdy jest globalna i jeszcze niezbudowana.
// Cykl (definicja w trakcie budowy) zgłasza z kodem cycleCode i zwraca nil.
func (l *loader) ensuredType(n *node, attr, v string, base bool, cycleCode string) Type {
	t, df := l.typeDef(n, attr, v, base)
	if df == nil {
		return t
	}
	if df.state == stateBuilding {
		l.errorf(cycleCode, n.attrPos(attr), "circular definition: %s refers to itself through %s/@%s", df.name, n.name(), attr)
		return nil
	}
	l.ensure(df)
	return t
}
