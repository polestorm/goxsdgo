// Package schema wczytuje dokumenty schematu XSD 1.0 i buduje z nich komponenty schematu (XSD 1.0 Second Edition,
// część 1, rozdziały 3 i 4): definicje typów, deklaracje elementów i atrybutów, grupy, symbole wieloznaczne,
// ograniczenia tożsamości i notacje. Sprawdza ograniczenia reprezentacji (src-*) i ograniczenia komponentów, które
// nie wymagają modeli zawartości (te sprawdza etap 4: cos-nonambig, cos-element-consistent, cos-particle-restrict,
// derivation-ok-restriction.5.4.2, cos-ct-extends.1.5, src-redefine.6.2.2).
//
// Pakiet nie sięga do sieci: dokumenty pochodzą wyłącznie z fs.FS podanego przez wywołującego (ADR 0006).
package schema

import (
	"github.com/polestorm/goxsdgo/internal/datatypes"
)

// Przestrzenie nazw o szczególnym znaczeniu.
const (
	NamespaceXSD = "http://www.w3.org/2001/XMLSchema"
	NamespaceXSI = "http://www.w3.org/2001/XMLSchema-instance"
	NamespaceXML = "http://www.w3.org/XML/1998/namespace"
)

// QName to nazwa komponentu: przestrzeń nazw i nazwa lokalna. Pusta przestrzeń oznacza jej brak (·absent·);
// pusty napis nie jest poprawną nazwą przestrzeni (3.15.2), więc nie ma kolizji.
type QName struct {
	Space, Local string
}

func (q QName) String() string {
	if q.Space == "" {
		return q.Local
	}
	return "{" + q.Space + "}" + q.Local
}

// Position wskazuje miejsce w dokumencie schematu, z którego pochodzi komponent albo błąd.
type Position struct {
	Document     string
	Line, Column int
}

// DerivationSet to zbiór metod wyprowadzenia: wartości {final}, {prohibited substitutions},
// {disallowed substitutions} i {substitution group exclusions}.
type DerivationSet uint8

const (
	DerivationExtension DerivationSet = 1 << iota
	DerivationRestriction
	DerivationSubstitution
	DerivationList
	DerivationUnion
)

// Has mówi, czy zbiór zawiera wszystkie metody z m.
func (s DerivationSet) Has(m DerivationSet) bool { return s&m == m }

// Type to definicja typu: *SimpleType albo *ComplexType.
type Type interface {
	// TypeName zwraca nazwę typu; Local jest puste dla typu anonimowego.
	TypeName() QName
	isType()
}

// SimpleType to definicja typu prostego (3.14). Ograniczenia na wartości trzyma typ z pakietu datatypes.
type SimpleType struct {
	Name QName
	// DT to typ danych: przestrzeń leksykalna, wartości i aspekty.
	DT *datatypes.Type
	// Base to {base type definition}: *SimpleType albo, dla anySimpleType, AnyType.
	Base Type
	// Item to {item type definition} listy; Members to {member type definitions} unii (po spłaszczeniu unii).
	Item    *SimpleType
	Members []*SimpleType
	Final   DerivationSet
	// Builtin: typ wbudowany z przestrzeni XSD (anySimpleType i 44 typy z części 2).
	Builtin bool
	Pos     Position
}

func (t *SimpleType) TypeName() QName { return t.Name }
func (*SimpleType) isType()           {}

// ContentKind to rodzaj {content type} typu złożonego.
type ContentKind uint8

const (
	ContentEmpty ContentKind = iota
	ContentSimple
	ContentElementOnly
	ContentMixed
)

// ComplexType to definicja typu złożonego (3.4).
type ComplexType struct {
	Name QName
	// Base to {base type definition}; dla anyType to sam anyType.
	Base Type
	// Derivation to {derivation method}: DerivationExtension albo DerivationRestriction.
	Derivation DerivationSet
	Abstract   bool
	Final      DerivationSet
	// Block to {prohibited substitutions}.
	Block DerivationSet
	// Attributes to {attribute uses} w ustalonej kolejności (własne, z grup atrybutów, odziedziczone).
	Attributes []*AttributeUse
	// AttributeWildcard to {attribute wildcard}.
	AttributeWildcard *Wildcard
	Content           ContentKind
	// SimpleContent to typ prosty treści, gdy Content == ContentSimple.
	SimpleContent *SimpleType
	// Particle to cząstka treści, gdy Content to ContentElementOnly albo ContentMixed.
	Particle *Particle
	// Model to skompilowany model zawartości (etap 4), gdy Content to ContentEmpty, ContentElementOnly albo
	// ContentMixed; nil dla treści prostej i dla schematu z błędami.
	Model *ContentModel
	Pos   Position
}

func (t *ComplexType) TypeName() QName { return t.Name }
func (*ComplexType) isType()           {}

// Unbounded to wartość MaxOccurs dla maxOccurs="unbounded".
const Unbounded = -1

// Particle to cząstka (3.9). Liczności większe niż int64 są obcięte do math.MaxInt64 (ADR 0006).
type Particle struct {
	MinOccurs int64
	// MaxOccurs to liczba albo Unbounded.
	MaxOccurs int64
	// Term to *ElementDecl, *ModelGroup albo *Wildcard.
	Term Term
	Pos  Position
}

// Term to {term} cząstki.
type Term interface{ isTerm() }

// Compositor to {compositor} grupy modelu.
type Compositor uint8

const (
	CompositorSequence Compositor = iota
	CompositorChoice
	CompositorAll
)

// ModelGroup to grupa modelu (3.8).
type ModelGroup struct {
	Compositor Compositor
	Particles  []*Particle
	Pos        Position
}

func (*ModelGroup) isTerm() {}

// ModelGroupDef to definicja grupy modelu (3.7).
type ModelGroupDef struct {
	Name  QName
	Group *ModelGroup
	Pos   Position
}

// Scope to {scope} deklaracji: globalna albo lokalna w typie złożonym (Parent) lub w grupie (Parent == nil).
type Scope struct {
	Global bool
	Parent *ComplexType
}

// ValueConstraint to {value constraint}: wartość domyślna albo stała.
type ValueConstraint struct {
	Fixed   bool
	Lexical string
	// Value to wartość w przestrzeni wartości typu, gdy typ jest prosty (albo ma treść prostą).
	Value datatypes.Value
	// ctx to element dokumentu schematu z atrybutem default/fixed: jego przestrzenie nazw rozwiązują wartości QName.
	ctx *node
}

// ElementDecl to deklaracja elementu (3.3).
type ElementDecl struct {
	Name     QName
	Type     Type
	Scope    Scope
	Nillable bool
	Value    *ValueConstraint
	// IdentityConstraints to {identity-constraint definitions}.
	IdentityConstraints []*IdentityConstraint
	// SubstitutionGroup to {substitution group affiliation}.
	SubstitutionGroup *ElementDecl
	// SubstitutionExclusions to {substitution group exclusions}; Block to {disallowed substitutions}.
	SubstitutionExclusions DerivationSet
	Block                  DerivationSet
	Abstract               bool
	Pos                    Position
}

func (*ElementDecl) isTerm() {}

// AttributeDecl to deklaracja atrybutu (3.2).
type AttributeDecl struct {
	Name  QName
	Type  *SimpleType
	Scope Scope
	Value *ValueConstraint
	Pos   Position
}

// AttributeUse to użycie atrybutu (3.5).
type AttributeUse struct {
	Required bool
	Decl     *AttributeDecl
	Value    *ValueConstraint
	Pos      Position
}

// EffectiveValue zwraca ·effective value constraint· użycia: własne albo z deklaracji.
func (u *AttributeUse) EffectiveValue() *ValueConstraint {
	if u.Value != nil {
		return u.Value
	}
	return u.Decl.Value
}

// AttributeGroup to definicja grupy atrybutów (3.6).
type AttributeGroup struct {
	Name     QName
	Uses     []*AttributeUse
	Wildcard *Wildcard
	Pos      Position
}

// ProcessContents to {process contents} symbolu wieloznacznego.
type ProcessContents uint8

const (
	ProcessStrict ProcessContents = iota
	ProcessLax
	ProcessSkip
)

// NamespaceKind to rodzaj {namespace constraint}.
type NamespaceKind uint8

const (
	NamespaceAny NamespaceKind = iota
	// NamespaceNot: para (not, Not); Not == "" oznacza negację braku przestrzeni.
	NamespaceNot
	// NamespaceSet: zbiór Set; "" w zbiorze oznacza brak przestrzeni.
	NamespaceSet
)

// NamespaceConstraint to {namespace constraint} (3.10.1).
type NamespaceConstraint struct {
	Kind NamespaceKind
	Not  string
	// Set jest posortowany i bez powtórzeń.
	Set []string
}

// Wildcard to symbol wieloznaczny (3.10).
type Wildcard struct {
	Namespaces NamespaceConstraint
	Process    ProcessContents
	Pos        Position
}

func (*Wildcard) isTerm() {}

// ICCategory to {identity-constraint category}.
type ICCategory uint8

const (
	ICUnique ICCategory = iota
	ICKey
	ICKeyRef
)

// IdentityConstraint to definicja ograniczenia tożsamości (3.11).
type IdentityConstraint struct {
	Name     QName
	Category ICCategory
	Selector *XPath
	Fields   []*XPath
	// Refer to {referenced key} dla keyref.
	Refer *IdentityConstraint
	Pos   Position
}

// Notation to deklaracja notacji (3.12).
type Notation struct {
	Name           QName
	Public, System string
	Pos            Position
}

// Schema to schemat złożony ze wszystkich wczytanych dokumentów (3.15). Mapy są indeksami komponentów
// globalnych; Order* zachowują kolejność z dokumentów dla powtarzalności.
type Schema struct {
	Types               map[QName]Type
	Elements            map[QName]*ElementDecl
	Attributes          map[QName]*AttributeDecl
	AttributeGroups     map[QName]*AttributeGroup
	Groups              map[QName]*ModelGroupDef
	Notations           map[QName]*Notation
	IdentityConstraints map[QName]*IdentityConstraint

	// ComplexTypes i ElementDecls to wszystkie zbudowane typy złożone i deklaracje elementów (także lokalne
	// i anonimowe) w kolejności budowy; etap 4 sprawdza na nich ograniczenia modeli zawartości.
	ComplexTypes []*ComplexType
	ElementDecls []*ElementDecl
	// Documents to wczytane dokumenty w kolejności wczytania.
	Documents []DocumentInfo
}

// DocumentInfo opisuje jeden wczytany dokument schematu: nazwę (ścieżka w fs.FS albo adres dokumentu
// z Config.Resolve), adres bezwzględny, pod którym go znaleziono (pusty, gdy go nie ma), źródło, rozmiar
// i SHA-256 przeczytanych bajtów.
type DocumentInfo struct {
	Name     string
	Location string
	Resolved bool
	Size     int64
	SHA256   [32]byte
}
