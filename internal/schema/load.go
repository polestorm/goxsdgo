package schema

import (
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"strings"
)

// Config ustawia wczytywanie schematu. Wartość zerowa to ustawienia domyślne: bez sieci, dokumenty tylko z fs.FS.
type Config struct {
	// Locations przypisuje adresom ze schemaLocation (dokładnie w postaci z dokumentu albo po rozwiązaniu względem
	// adresu dokumentu, który go zawiera) ścieżki w fs.FS.
	Locations map[string]string
	// Namespaces przypisuje przestrzeniom nazw ścieżki dokumentów schematu dla importów bez schemaLocation albo
	// z adresem, którego nie da się rozwiązać (strategia 3 z 4.3.2).
	Namespaces map[string]string
	// MaxDocuments ogranicza liczbę wczytanych dokumentów schematu (domyślnie 1000), MaxDepth zagnieżdżenie
	// include/import/redefine (domyślnie 64). Zero oznacza wartość domyślną.
	MaxDocuments, MaxDepth int
}

// defKind to przestrzeń symboli komponentu globalnego (2.5, 3.15.3).
type defKind uint8

const (
	kindType defKind = iota
	kindElement
	kindAttribute
	kindAttributeGroup
	kindGroup
	kindNotation
	kindIdentity
	kindCount
)

var kindNames = [...]string{"type definition", "element declaration", "attribute declaration",
	"attribute group definition", "model group definition", "notation declaration", "identity-constraint definition"}

// def to komponent globalny przed zbudowaniem i po nim: element dokumentu schematu, z którego pochodzi, i komponent.
type def struct {
	kind defKind
	name QName
	node *node
	// orig to komponent redefiniowany, gdy def pochodzi z xs:redefine (4.2.2).
	orig  *def
	state buildState
	comp  any
}

type buildState uint8

const (
	stateNew buildState = iota
	stateBuilding
	stateBuilt
)

// document to wczytany dokument schematu.
type document struct {
	path string
	// base to bezwzględny adres dokumentu, gdy wczytano go przez Config.Locations; służy do rozwiązywania
	// względnych schemaLocation.
	base string
	root *node
	// tns to docelowa przestrzeń nazw po ewentualnej zamianie kameleona (4.2.1, punkt 3.2); hasTNS mówi, czy
	// dokument ma atrybut targetNamespace.
	tns         string
	hasTNS      bool
	imports     map[string]bool
	importsNoNS bool
	// included to dokumenty włączone przez include (bezpośrednio i pośrednio) — redefine szuka w nich komponentów.
	included                   []*document
	elementQualified           bool
	attributeQualified         bool
	blockDefault, finalDefault DerivationSet
}

type loader struct {
	fsys fs.FS
	cfg  Config
	errs ErrorList
	// unsupported: pierwszy błąd konstrukcji spoza zakresu.
	unsupported *Error
	docs        []*document
	loaded      map[string]*document
	// ownTNS: ścieżka → targetNamespace dokumentu z pliku (pusty, gdy go nie ma), znany po pierwszym wczytaniu.
	ownTNS map[string]string
	// dirs: zawartość katalogów fs.FS (do sprawdzania wielkości liter w ścieżkach).
	dirs  map[string]map[string]bool
	index [kindCount]map[QName]*def
	// defOf: element najwyższego poziomu (dziecko schema albo redefine) → jego def.
	defOf map[*node]*def
	sch   *Schema
	// queue to odłożone budowanie typów anonimowych (bez fałszywych cykli przez deklaracje elementów).
	queue []func()
	// checks to kontrole końcowe, uruchamiane po zbudowaniu wszystkiego.
	checks []func()
	// notationTypes: typy pochodne od NOTATION z elementem restriction, którego kontekst rozwiązuje wartości.
	notationTypes []notationType
	simpleTypes   []*SimpleType
	attrDecls     []*AttributeDecl
	attrUses      []*AttributeUse
}

type notationType struct {
	st *SimpleType
	n  *node
}

func (l *loader) errorf(code string, pos Position, format string, args ...any) {
	l.errs = append(l.errs, &Error{Code: code, Pos: pos, Message: fmt.Sprintf(format, args...)})
}

// Load wczytuje schemat z dokumentów names w fsys. Zwraca ErrorList z błędami schematu albo błąd opakowujący
// errors.ErrUnsupported dla konstrukcji spoza obsługiwanego zakresu.
func Load(fsys fs.FS, cfg Config, names ...string) (*Schema, error) {
	if cfg.MaxDocuments == 0 {
		cfg.MaxDocuments = 1000
	}
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 64
	}
	l := &loader{fsys: fsys, cfg: cfg, loaded: map[string]*document{}, defOf: map[*node]*def{},
		ownTNS: map[string]string{}, dirs: map[string]map[string]bool{}}
	for k := range l.index {
		l.index[k] = map[QName]*def{}
	}
	l.sch = &Schema{
		Types: map[QName]Type{}, Elements: map[QName]*ElementDecl{}, Attributes: map[QName]*AttributeDecl{},
		AttributeGroups: map[QName]*AttributeGroup{}, Groups: map[QName]*ModelGroupDef{},
		Notations: map[QName]*Notation{}, IdentityConstraints: map[QName]*IdentityConstraint{},
	}
	if len(names) == 0 {
		l.errorf(CodeNotSchema, Position{}, "no schema documents given")
	}
	for _, name := range names {
		p := path.Clean(name)
		if !fs.ValidPath(p) {
			l.errorf(CodeNotSchema, Position{Document: name}, "schema document path %q is not a valid fs.FS path", name)
			continue
		}
		if _, err := fs.Stat(fsys, p); err != nil {
			l.errorf(CodeNotSchema, Position{Document: name}, "schema document %s cannot be read: %v", name, err)
			continue
		}
		l.loadDocument(p, "", roleTop, nil, nil, 0)
	}
	if l.unsupported != nil {
		return nil, l.unsupported
	}
	l.build()
	if l.unsupported != nil {
		return nil, l.unsupported
	}
	if len(l.errs) > 0 {
		return nil, l.errs
	}
	return l.sch, nil
}

// role to sposób, w jaki dokument trafił do schematu; decyduje o kodach błędów i o zamianie kameleona.
type role uint8

const (
	roleTop role = iota
	roleInclude
	roleRedefine
	roleImport
)

func (r role) code() string {
	switch r {
	case roleInclude:
		return CodeInclude
	case roleRedefine:
		return CodeRedefine
	case roleImport:
		return CodeImport
	}
	return CodeNotSchema
}

// loadDocument wczytuje dokument p. from to dokument zawierający include/import/redefine (nil dla dokumentów
// podanych wprost), ref to ten element. Zwraca dokument albo nil, gdy go nie wczytano.
func (l *loader) loadDocument(p, base string, r role, from *document, ref *node, depth int) *document {
	// Klucz dokumentu to ścieżka i docelowa przestrzeń po zamianie kameleona: ten sam plik podany wprost
	// i włączony przez include daje te same komponenty, a plik bez targetNamespace włączony przez dwa dokumenty
	// o różnych przestrzeniach daje dwa komplety komponentów (4.2.1).
	effective := func(own string) string {
		if own == "" && (r == roleInclude || r == roleRedefine) {
			return from.tns
		}
		return own
	}
	if own, known := l.ownTNS[p]; known {
		if d, ok := l.loaded[p+"\x00"+effective(own)]; ok {
			switch r {
			case roleInclude, roleRedefine:
				l.checkNamespaceMatch(d, r, from, ref)
			case roleImport:
				l.checkImportNamespace(d, ref)
			}
			return d
		}
	}
	refPos := Position{Document: p}
	if ref != nil {
		refPos = ref.pos()
	}
	if len(l.docs) >= l.cfg.MaxDocuments {
		l.errorf(CodeLimit, refPos, "more than %d schema documents", l.cfg.MaxDocuments)
		return nil
	}
	if depth > l.cfg.MaxDepth {
		l.errorf(CodeLimit, refPos, "include/import/redefine nested deeper than %d", l.cfg.MaxDepth)
		return nil
	}
	f, err := l.fsys.Open(p)
	if err != nil {
		if r == roleTop {
			l.errorf(CodeNotSchema, refPos, "schema document %s cannot be read: %v", p, err)
		}
		return nil
	}
	root, perr := parseDocument(f, p)
	f.Close()
	if perr != nil {
		if e, ok := perr.(*Error); ok {
			if e.Cause == errUnsupported && l.unsupported == nil {
				l.unsupported = &Error{Code: e.Code, Pos: e.Pos, Message: e.Message, Cause: errUnsupported}
				return nil
			}
			l.errs = append(l.errs, e)
			if r != roleTop {
				l.errorf(r.code()+".1", refPos, "%s does not resolve to a well-formed schema document", p)
			}
		}
		return nil
	}
	if root == nil || !root.is("schema") {
		pos := refPos
		if root != nil && r == roleTop {
			pos = Position{Document: p, Line: root.line, Column: root.col}
		}
		l.errorf(r.code()+".1", pos, "%s is not a schema document (root element is not xs:schema)", p)
		return nil
	}
	tns, hasTNS := root.attr("targetNamespace")
	tns = collapse(tns)
	if hasTNS && tns == "" {
		// Pusty napis nie jest nazwą przestrzeni; targetNamespace="" jest niespójne, a nie tym samym co brak
		// atrybutu (3.15.2). Dalej traktujemy dokument jak dokument bez przestrzeni.
		pos := root.attrPos("targetNamespace")
		pos.Document = p
		l.errorf(CodeSchemaProps+".1", pos, "targetNamespace must not be empty; omit it for a schema without a target namespace")
		hasTNS = false
	}
	l.ownTNS[p] = tns
	key := p + "\x00" + effective(tns)
	if d, ok := l.loaded[key]; ok {
		return d
	}
	d := &document{path: p, base: base, root: root, imports: map[string]bool{}, hasTNS: hasTNS, tns: effective(tns)}
	setDoc(root, d)
	l.loaded[key] = d
	l.docs = append(l.docs, d)
	l.sch.Documents = append(l.sch.Documents, p)

	l.checkSFS(root, map[string]bool{})
	switch r {
	case roleInclude, roleRedefine:
		l.checkNamespaceMatch(d, r, from, ref)
	case roleImport:
		l.checkImportNamespace(d, ref)
	}
	if v, ok := root.attr("elementFormDefault"); ok && collapse(v) == "qualified" {
		d.elementQualified = true
	}
	if v, ok := root.attr("attributeFormDefault"); ok && collapse(v) == "qualified" {
		d.attributeQualified = true
	}
	if v, ok := root.attr("blockDefault"); ok {
		d.blockDefault = parseDerivationSet(v)
	}
	if v, ok := root.attr("finalDefault"); ok {
		d.finalDefault = parseDerivationSet(v)
	}
	// Importy znane przed rozwiązywaniem nazw: zbieramy je z całego dokumentu.
	for _, k := range root.kids {
		if k.is("import") {
			ns, hasNS := k.attr("namespace")
			ns = collapse(ns)
			if hasNS && ns != "" {
				d.imports[ns] = true
				if d.hasTNS && ns == d.tns {
					l.errorf(CodeImport+".1.1", k.pos(), "a schema document cannot import its own target namespace %q", ns)
				}
			} else {
				d.importsNoNS = true
				if !d.hasTNS {
					l.errorf(CodeImport+".1.2", k.pos(), "import without namespace requires a targetNamespace on the importing schema")
				}
			}
		}
	}
	// Składanie: include, import, redefine w kolejności z dokumentu.
	for _, k := range root.kids {
		switch {
		case k.is("include"):
			loc, _ := k.attr("schemaLocation")
			if q, b, ok := l.locate(d, loc); ok {
				if inc := l.loadDocument(q, b, roleInclude, d, k, depth+1); inc != nil {
					d.included = append(d.included, inc)
					d.included = append(d.included, inc.included...)
				}
			}
		case k.is("import"):
			ns, _ := k.attr("namespace")
			ns = collapse(ns)
			if ns == NamespaceXSD {
				continue // komponenty przestrzeni XSD są wbudowane
			}
			loc, hasLoc := k.attr("schemaLocation")
			q, b, ok := "", "", false
			if hasLoc {
				q, b, ok = l.locate(d, loc)
			}
			if !ok {
				if np, has := l.cfg.Namespaces[ns]; has && fs.ValidPath(np) {
					q, b, ok = np, "", true
				}
			}
			if ok {
				l.loadDocument(q, b, roleImport, d, k, depth+1)
			}
		case k.is("redefine"):
			loc, _ := k.attr("schemaLocation")
			var red *document
			if q, b, ok := l.locate(d, loc); ok {
				red = l.loadDocument(q, b, roleRedefine, d, k, depth+1)
			}
			l.applyRedefine(d, k, red)
		}
	}
	// Komponenty najwyższego poziomu.
	for _, k := range root.kids {
		if k.space != NamespaceXSD {
			continue
		}
		l.registerTop(d, k)
	}
	l.registerIdentity(d, root)
	return d
}

func setDoc(n *node, d *document) {
	n.doc = d
	for _, k := range n.kids {
		setDoc(k, d)
	}
}

// checkImportNamespace sprawdza punkt 3 z Import Constraints and Semantics (4.2.3).
func (l *loader) checkImportNamespace(d *document, ref *node) {
	ns, hasNS := ref.attr("namespace")
	ns = collapse(ns)
	if hasNS && ns != "" {
		if !d.hasTNS || d.tns != ns {
			l.errorf(CodeImport+".3.1", ref.pos(), "imported document %s has targetNamespace %q, the import names %q", d.path, d.tns, ns)
		}
	} else if d.hasTNS {
		l.errorf(CodeImport+".3.2", ref.pos(), "import without namespace refers to %s, which has targetNamespace %q", d.path, d.tns)
	}
}

// checkNamespaceMatch sprawdza punkt 2 z 4.2.1 (include) albo punkt 3 z 4.2.2 (redefine).
func (l *loader) checkNamespaceMatch(d *document, r role, from *document, ref *node) {
	if !d.hasTNS {
		return // 2.2 albo 2.3: kameleon albo oba bez przestrzeni
	}
	// Porównujemy przestrzeń po zamianie kameleona: dokument włączony jako kameleon ma docelową przestrzeń
	// włączającego, choć nie ma atrybutu targetNamespace.
	if d.tns != from.tns {
		clause := ".2.1"
		if r == roleRedefine {
			clause = ".3.1"
		}
		l.errorf(r.code()+clause, ref.pos(), "%s has targetNamespace %q, the including document %q", d.path, d.tns, from.tns)
	}
}

// locate rozwiązuje schemaLocation dokumentu d bez sieci: najpierw Config.Locations (adres dosłowny i po
// rozwiązaniu względem adresu dokumentu), potem ścieżka względna w fs.FS. Zwraca ścieżkę i adres bazowy nowego
// dokumentu.
func (l *loader) locate(d *document, loc string) (string, string, bool) {
	loc = collapse(loc)
	if i := strings.IndexByte(loc, '#'); i >= 0 {
		loc = loc[:i]
	}
	if loc == "" {
		return "", "", false
	}
	if p, ok := l.cfg.Locations[loc]; ok && fs.ValidPath(p) {
		return p, absoluteOrEmpty(loc), true
	}
	abs := ""
	if d.base != "" {
		if b, err := url.Parse(d.base); err == nil {
			if r, err := url.Parse(loc); err == nil {
				abs = b.ResolveReference(r).String()
				if p, ok := l.cfg.Locations[abs]; ok && fs.ValidPath(p) {
					return p, abs, true
				}
			}
		}
	}
	u, err := url.Parse(loc)
	if err != nil || u.Scheme != "" || u.Host != "" || strings.HasPrefix(loc, "/") {
		return "", "", false // adres bezwzględny bez wpisu w Locations: sieci nie używamy
	}
	rel, err := url.PathUnescape(u.Path)
	if err != nil {
		rel = u.Path
	}
	p := path.Join(path.Dir(d.path), rel)
	if !fs.ValidPath(p) || !l.exists(p) {
		return "", "", false
	}
	return p, abs, true
}

// exists sprawdza, czy plik p istnieje w fs.FS z dokładnie tą wielkością liter na każdym poziomie ścieżki.
// System plików bez rozróżniania wielkości liter (macOS, Windows) otworzyłby „Schz012_b.xsd” jako
// „schZ012_b.xsd”; wynik nie może zależeć od systemu.
func (l *loader) exists(p string) bool {
	dir := "."
	for _, part := range strings.Split(p, "/") {
		entries, ok := l.dirs[dir]
		if !ok {
			entries = map[string]bool{}
			list, err := fs.ReadDir(l.fsys, dir)
			if err != nil {
				return false
			}
			for _, e := range list {
				entries[e.Name()] = true
			}
			l.dirs[dir] = entries
		}
		if !entries[part] {
			return false
		}
		dir = path.Join(dir, part)
	}
	return true
}

func absoluteOrEmpty(loc string) string {
	if u, err := url.Parse(loc); err == nil && u.Scheme != "" {
		return loc
	}
	return ""
}

func parseDerivationSet(v string) DerivationSet {
	var s DerivationSet
	for _, t := range strings.Fields(v) {
		switch t {
		case "#all":
			s = DerivationExtension | DerivationRestriction | DerivationSubstitution | DerivationList | DerivationUnion
		case "extension":
			s |= DerivationExtension
		case "restriction":
			s |= DerivationRestriction
		case "substitution":
			s |= DerivationSubstitution
		case "list":
			s |= DerivationList
		case "union":
			s |= DerivationUnion
		}
	}
	return s
}

// registerTop zapisuje komponent najwyższego poziomu dokumentu d w indeksie.
func (l *loader) registerTop(d *document, n *node) {
	var kind defKind
	switch n.local {
	case "simpleType", "complexType":
		kind = kindType
	case "element":
		kind = kindElement
	case "attribute":
		kind = kindAttribute
	case "attributeGroup":
		kind = kindAttributeGroup
	case "group":
		kind = kindGroup
	case "notation":
		kind = kindNotation
	default:
		return
	}
	name, ok := n.attr("name")
	if !ok {
		return // brak nazwy zgłasza kontrola według schematu dla schematów
	}
	q := QName{Space: d.tns, Local: collapse(name)}
	df := &def{kind: kind, name: q, node: n}
	df.comp = newComponent(kind, q, n)
	l.put(df)
}

func newComponent(kind defKind, q QName, n *node) any {
	switch kind {
	case kindType:
		if n.local == "simpleType" {
			return &SimpleType{Name: q, Pos: n.pos()}
		}
		return &ComplexType{Name: q, Pos: n.pos()}
	case kindElement:
		return &ElementDecl{Name: q, Scope: Scope{Global: true}, Pos: n.pos()}
	case kindAttribute:
		return &AttributeDecl{Name: q, Scope: Scope{Global: true}, Pos: n.pos()}
	case kindAttributeGroup:
		return &AttributeGroup{Name: q, Pos: n.pos()}
	case kindGroup:
		return &ModelGroupDef{Name: q, Group: &ModelGroup{Pos: n.pos()}, Pos: n.pos()}
	case kindNotation:
		return &Notation{Name: q, Pos: n.pos()}
	case kindIdentity:
		return &IdentityConstraint{Name: q, Pos: n.pos()}
	}
	return nil
}

func (l *loader) put(df *def) {
	if old, ok := l.index[df.kind][df.name]; ok && old.node != df.node {
		l.errorf(CodeSchemaProps+".2", df.node.pos(), "duplicate %s %s (also at %s:%d)", kindNames[df.kind],
			df.name, old.node.doc.path, old.node.line)
		return
	}
	l.index[df.kind][df.name] = df
	l.defOf[df.node] = df
}

// registerIdentity zapisuje ograniczenia tożsamości z całego dokumentu: ich nazwy są globalne (3.11.1), a keyref
// może wskazywać ograniczenie zdefiniowane dalej.
func (l *loader) registerIdentity(d *document, n *node) {
	for _, k := range n.kids {
		if k.space != NamespaceXSD {
			continue
		}
		if (k.local == "key" || k.local == "unique" || k.local == "keyref") && n.is("element") {
			if name, ok := k.attr("name"); ok {
				q := QName{Space: d.tns, Local: collapse(name)}
				df := &def{kind: kindIdentity, name: q, node: k}
				df.comp = newComponent(kindIdentity, q, k)
				l.put(df)
			}
			continue
		}
		if k.local == "annotation" {
			continue
		}
		l.registerIdentity(d, k)
	}
}

// applyRedefine stosuje redefinicje z elementu rn dokumentu d do komponentów dokumentu red (4.2.2).
func (l *loader) applyRedefine(d *document, rn *node, red *document) {
	if red == nil {
		if len(rn.xsdKids()) > 0 {
			l.errorf(CodeRedefine+".1", rn.pos(), "schemaLocation of xs:redefine does not resolve, but it redefines components")
		}
		return
	}
	inRed := map[*document]bool{red: true}
	for _, x := range red.included {
		inRed[x] = true
	}
	for _, c := range rn.kids {
		if c.space != NamespaceXSD || c.local == "annotation" {
			continue
		}
		name, ok := c.attr("name")
		if !ok {
			continue
		}
		var kind defKind
		switch c.local {
		case "simpleType", "complexType":
			kind = kindType
		case "group":
			kind = kindGroup
		case "attributeGroup":
			kind = kindAttributeGroup
		default:
			continue
		}
		q := QName{Space: d.tns, Local: collapse(name)}
		old := l.index[kind][q]
		if old == nil || !inRed[old.node.doc] || old.node.local != c.local {
			l.errorf(CodeRedefine, c.pos(), "%s %s redefines a component that the redefined document %s does not define",
				c.name(), q, red.path)
			continue
		}
		// old może sam pochodzić z redefinicji (łańcuch: A redefiniuje B, które redefiniuje C); wtedy oryginałem
		// nowej redefinicji jest redefinicja z B.
		nd := &def{kind: kind, name: q, node: c, orig: old}
		nd.comp = newComponent(kind, q, c)
		l.index[kind][q] = nd
		l.defOf[c] = nd
		// Oryginał zostaje bez nazwy (4.2.2, Individual Component Redefinition 1.1).
		switch o := old.comp.(type) {
		case *SimpleType:
			o.Name = QName{}
		case *ComplexType:
			o.Name = QName{}
		}
		l.checkRedefineShape(c, q)
	}
}

// checkRedefineShape sprawdza punkty 5, 6.1 i 7.1 z Redefinition Constraints and Semantics.
func (l *loader) checkRedefineShape(c *node, q QName) {
	switch c.local {
	case "simpleType":
		r := c.kid("restriction")
		if r == nil {
			l.errorf(CodeRedefine+".5", c.pos(), "a redefined simple type must be a restriction of itself")
			return
		}
		if b, ok := r.attr("base"); !ok || !l.refersTo(r, b, q) {
			l.errorf(CodeRedefine+".5", r.pos(), "a redefined simple type must restrict its original %s", q)
		}
	case "complexType":
		var der *node
		for _, k := range c.xsdKids() {
			if k.local == "simpleContent" || k.local == "complexContent" {
				for _, g := range k.xsdKids() {
					der = g
				}
			}
		}
		if der == nil {
			l.errorf(CodeRedefine+".5", c.pos(), "a redefined complex type must restrict or extend its original")
			return
		}
		if b, ok := der.attr("base"); !ok || !l.refersTo(der, b, q) {
			l.errorf(CodeRedefine+".5", der.pos(), "a redefined complex type must be derived from its original %s", q)
		}
	case "group":
		refs := selfRefs(c, "group", q, l)
		if len(refs) > 1 {
			l.errorf(CodeRedefine+".6.1.1", c.pos(), "a redefined group may refer to itself only once")
		}
		for _, r := range refs {
			if mn, ok := r.attr("minOccurs"); ok && collapse(mn) != "1" {
				l.errorf(CodeRedefine+".6.1.2", r.pos(), "the self-reference in a redefined group must have minOccurs 1")
			}
			if mx, ok := r.attr("maxOccurs"); ok && collapse(mx) != "1" {
				l.errorf(CodeRedefine+".6.1.2", r.pos(), "the self-reference in a redefined group must have maxOccurs 1")
			}
		}
		// Punkt 6.2.2 (grupa bez odwołania do siebie musi być poprawną restrykcją oryginału) wymaga sprawdzenia
		// restrykcji cząstek: etap 4 (cos-particle-restrict).
	case "attributeGroup":
		refs := selfRefs(c, "attributeGroup", q, l)
		if len(refs) > 1 {
			l.errorf(CodeRedefine+".7.1", c.pos(), "a redefined attribute group may refer to itself only once")
		}
	}
}

// refersTo mówi, czy wartość QName v w kontekście n wskazuje nazwę q.
func (l *loader) refersTo(n *node, v string, q QName) bool {
	got, ok := l.qname(n, v)
	return ok && got == q
}

// selfRefs zwraca elementy local z atrybutem ref wskazującym q w poddrzewie c.
func selfRefs(c *node, local string, q QName, l *loader) []*node {
	var out []*node
	var walk func(n *node)
	walk = func(n *node) {
		for _, k := range n.kids {
			if k.is(local) {
				if v, ok := k.attr("ref"); ok && l.refersTo(k, v, q) {
					out = append(out, k)
				}
			}
			walk(k)
		}
	}
	walk(c)
	return out
}
