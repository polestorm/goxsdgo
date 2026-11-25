package xsd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/polestorm/goxsdgo/internal/schema"
	"github.com/polestorm/goxsdgo/internal/validate"
	"github.com/polestorm/goxsdgo/internal/xmlchars"
	"github.com/polestorm/goxsdgo/internal/xmlreader"
)

// DocumentLimits to limity czytania walidowanego dokumentu (Config.Document). Chronią przed dokumentami
// złośliwymi: każdy limit ogranicza pamięć albo czas niezależnie od schematu (ADR 0004, ADR 0011).
type DocumentLimits struct {
	MaxDepth           int // głębokość zagnieżdżenia elementów
	MaxNameLength      int // długość nazwy elementu albo atrybutu w bajtach UTF-8
	MaxAttributes      int // atrybuty jednego elementu, łącznie z deklaracjami przestrzeni nazw
	MaxTextSize        int // tekst jednego elementu albo atrybutu, komentarza, PI, w bajtach
	MaxEntityExpansion int // łączna długość tekstu z rozwiniętych encji wewnętrznych w dokumencie, w bajtach
	MaxEntityDepth     int // zagnieżdżenie odwołań do encji
	MaxDTDSize         int // długość deklaracji DOCTYPE z podzbiorem wewnętrznym, w bajtach
}

// DefaultDocumentLimits zwraca domyślne limity czytania dokumentu: głębokość 512, nazwa 16 KiB, 4096 atrybutów,
// tekst 16 MiB, rozwinięcia encji 4 MiB, zagnieżdżenie encji 32, DOCTYPE 1 MiB.
func DefaultDocumentLimits() DocumentLimits {
	l := xmlreader.DefaultLimits()
	return DocumentLimits{MaxDepth: l.MaxDepth, MaxNameLength: l.MaxNameLength, MaxAttributes: l.MaxAttributes,
		MaxTextSize: l.MaxTextSize, MaxEntityExpansion: l.MaxEntityExpansion, MaxEntityDepth: l.MaxEntityDepth,
		MaxDTDSize: l.MaxDTDSize}
}

// ErrLimit oznacza przekroczenie limitu biblioteki podczas walidacji (głębokość, rozmiar tekstu, liczba wpisów
// ID i ograniczeń tożsamości, złożoność dopasowania modelu zawartości). To nie jest werdykt o dokumencie.
var ErrLimit = validate.ErrLimit

// ErrTooManyErrors oznacza, że walidację przerwano po Config.MaxErrors błędach. Validate zwraca go razem
// z zebranymi błędami; dokument jest wtedy na pewno błędny.
var ErrTooManyErrors = validate.ErrTooManyErrors

// Config ustawia kompilację schematu i walidację. Wartość zerowa to ustawienia domyślne: bez sieci, schematy tylko
// z fsys, wskazówki xsi:schemaLocation wyłączone.
//
// Kształt API jest wstępny (pytanie 5 w docs/adr/open-questions.md); ustalą go testy kolejnych etapów.
type Config struct {
	// Locations przypisuje adresom ze schemaLocation (w include, import i redefine) ścieżki w fsys: adresowi
	// dosłownie jak w dokumencie albo po rozwiązaniu względem adresu dokumentu, który go zawiera. Schematy MF
	// importują pliki pod adresami http://crd.gov.pl/..., a biblioteka nigdy nie sięga do sieci. Adres względny
	// bez wpisu jest rozwiązywany względem ścieżki dokumentu w fsys.
	Locations map[string]string
	// Namespaces przypisuje przestrzeniom nazw ścieżki dokumentów schematu w fsys; używa ich import bez
	// schemaLocation albo z adresem, którego nie da się rozwiązać (ADR 0006).
	Namespaces map[string]string
	// MaxDocuments i MaxDepth ograniczają liczbę wczytanych dokumentów schematu (domyślnie 1000) i zagnieżdżenie
	// include, import i redefine (domyślnie 64). Zero oznacza wartość domyślną.
	MaxDocuments, MaxDepth int

	// SchemaLocationHints włącza wskazówki xsi:schemaLocation i xsi:noNamespaceSchemaLocation w dokumentach
	// (4.3.2). Wskazane dokumenty schematu są czytane wyłącznie z fsys: przez Locations albo jako ścieżka
	// względna wobec nazwy dokumentu podanej w ValidateDocument. Domyślnie wyłączone (ADR 0001, ADR 0008).
	SchemaLocationHints bool
	// MaxErrors przerywa walidację dokumentu po tylu błędach (ErrTooManyErrors). 0: 1000; ujemna: bez limitu.
	MaxErrors int
	// MaxIdentityEntries ogranicza łączną liczbę wartości ID, IDREF i wpisów tabel ograniczeń tożsamości
	// przechowywanych podczas walidacji jednego dokumentu. 0: 4 194 304.
	MaxIdentityEntries int

	// OnValue, jeśli nie nil, dostaje każdą poprawną wartość typu prostego z dokumentu: treść elementu o typie
	// prostym albo treści prostej i wartość atrybutu, z typem i postacią kanoniczną (pytanie 6, ADR 0009).
	// Wywołania są synchroniczne, w kolejności dokumentu, z goroutine wywołującej Validate. Bez funkcji walidacja
	// nie liczy nic ponad to, co sama potrzebuje.
	OnValue func(TypedValue)

	// Document to limity czytania walidowanego dokumentu (głębokość, nazwy, atrybuty, tekst, encje, DOCTYPE).
	// Wartość zerowa pola oznacza wartość domyślną z DefaultDocumentLimits, ujemna brak limitu. Przekroczenie
	// limitu kończy walidację błędem ErrLimit, nie błędem dokumentu (ADR 0011).
	Document DocumentLimits

	// NameRules wybiera reguły znaków nazw dla typów Name, NCName, NMTOKEN(S), ID, IDREF(S), ENTITY(IES), QName
	// i NOTATION oraz dla klas \i, \c (i \I, \C) we wzorcach: w schemacie i w walidowanym dokumencie. Wartość
	// zerowa NamesXML10Edition2 to dosłowny odsyłacz XSD 1.0 2e; NamesXML10Edition5 to reguły, według których
	// czytnik czyta nazwy elementów i atrybutów (ADR 0012).
	NameRules NameRules
}

// NameRules to reguły znaków nazw (ADR 0012).
type NameRules uint8

const (
	// NamesXML10Edition2: XML 1.0 Second Edition, dodatek B (domyślnie). Tak samo jak libxml2 dla wartości typów
	// nazw.
	NamesXML10Edition2 NameRules = iota
	// NamesXML10Edition5: XML 1.0 Fifth Edition, produkcje [4] i [4a]; szerszy zbiór znaków, jak w nazwach
	// elementów i atrybutów dokumentu i w XSD 1.1.
	NamesXML10Edition5
)

func (n NameRules) edition() xmlchars.Edition {
	if n == NamesXML10Edition5 {
		return xmlchars.Edition5
	}
	return xmlchars.Edition2
}

// TypedValue to poprawna wartość typu prostego w dokumencie, przekazywana do Config.OnValue.
type TypedValue struct {
	// Path to ścieżka elementu albo atrybutu, jak w Error.Path; Name to nazwa lokalna; Attribute: wartość atrybutu.
	Path, Name string
	Attribute  bool
	// Type to nazwa typu ("{przestrzeń}nazwa" albo "nazwa") albo "" dla typu anonimowego; Builtin to nazwa
	// najbliższego typu wbudowanego, z którego typ jest wyprowadzony (np. "decimal", "date", "anySimpleType").
	Type, Builtin string
	// Canonical to wartość w postaci kanonicznej typu (część 2 rekomendacji), np. "12.5" dla " 012.50 ".
	Canonical string
	// Line i Column to pozycja jak w Error: koniec znacznika początkowego elementu albo atrybut.
	Line, Column int
}

// Compile kompiluje schemat złożony z dokumentów names z fsys, z ustawieniami domyślnymi.
func Compile(fsys fs.FS, names ...string) (*Set, error) {
	return Config{}.Compile(fsys, names...)
}

// Compile kompiluje schemat złożony z dokumentów names z fsys. Gdy schemat nie jest poprawny (rozdziały 3 i 4
// części 1 rekomendacji), zwraca *SchemaError z listą błędów. Błąd opakowujący errors.ErrUnsupported oznacza
// konstrukcję, której biblioteka jeszcze nie obsługuje; nie mówi nic o poprawności schematu. Z włączonymi
// wskazówkami (SchemaLocationHints) names może być puste: schemat wskażą dopiero dokumenty.
func (c Config) Compile(fsys fs.FS, names ...string) (*Set, error) {
	s := &Set{cfg: c, fsys: fsys, names: append([]string(nil), names...)}
	if len(names) == 0 && c.SchemaLocationHints {
		s.schema = schema.Empty()
		return s, nil
	}
	sch, err := schema.Load(fsys, c.schemaConfig(), names...)
	if err != nil {
		return nil, schemaError(err)
	}
	s.schema = sch
	return s, nil
}

func (c Config) schemaConfig() schema.Config {
	return schema.Config{Locations: c.Locations, Namespaces: c.Namespaces, MaxDocuments: c.MaxDocuments,
		MaxDepth: c.MaxDepth, NameRules: c.NameRules.edition()}
}

func schemaError(err error) error {
	var list schema.ErrorList
	if !errors.As(err, &list) {
		return err
	}
	se := &SchemaError{}
	for _, e := range list {
		se.Errors = append(se.Errors, Error{Code: e.Code, Document: e.Pos.Document, Line: e.Pos.Line,
			Column: e.Pos.Column, Message: e.Message})
	}
	return se
}

// SchemaError to błąd kompilacji: schemat nie jest poprawny. Errors są w powtarzalnej kolejności.
type SchemaError struct {
	Errors []Error
}

func (e *SchemaError) Error() string {
	if len(e.Errors) == 0 {
		return "xsd: invalid schema"
	}
	var b strings.Builder
	b.WriteString("xsd: invalid schema: ")
	b.WriteString(e.Errors[0].Error())
	if len(e.Errors) > 1 {
		fmt.Fprintf(&b, " (and %d more)", len(e.Errors)-1)
	}
	return b.String()
}

// Set to skompilowany schemat, gotowy do walidacji wielu dokumentów, także równocześnie.
type Set struct {
	schema *schema.Schema
	cfg    Config
	fsys   fs.FS
	names  []string

	// hinted: schematy poszerzone o dokumenty ze wskazówek, według posortowanej listy ścieżek.
	mu     sync.Mutex
	hinted map[string]hintedSchema
}

type hintedSchema struct {
	sch *schema.Schema
	err error
}

// Validate waliduje dokument z r strumieniowo. Pusta lista błędów oznacza dokument poprawny. Błąd zwracany jako
// drugi wynik dotyczy wejścia albo ograniczeń biblioteki (ErrLimit, errors.ErrUnsupported), nie treści dokumentu;
// wyjątkiem jest ErrTooManyErrors, zwracany razem z błędami dokumentu.
func (s *Set) Validate(r io.Reader) ([]Error, error) {
	return s.ValidateDocument(r, "")
}

// ValidateDocument działa jak Validate; name to ścieżka dokumentu w fsys schematu, względem której są
// rozwiązywane względne wskazówki xsi:schemaLocation (gdy Config.SchemaLocationHints). Bez wskazówek name służy
// tylko komunikatom i może być puste.
func (s *Set) ValidateDocument(r io.Reader, name string) ([]Error, error) {
	d := s.cfg.Document
	opts := validate.Options{MaxErrors: s.cfg.MaxErrors, MaxIdentityEntries: s.cfg.MaxIdentityEntries,
		MaxTextSize: d.MaxTextSize, Reader: xmlreader.Limits{MaxDepth: d.MaxDepth, MaxNameLength: d.MaxNameLength,
			MaxAttributes: d.MaxAttributes, MaxTextSize: d.MaxTextSize, MaxEntityExpansion: d.MaxEntityExpansion,
			MaxEntityDepth: d.MaxEntityDepth, MaxDTDSize: d.MaxDTDSize}, Names: s.cfg.NameRules.edition()}
	if f := s.cfg.OnValue; f != nil {
		opts.OnValue = func(ev validate.ValueEvent) {
			f(TypedValue{Path: ev.Path, Name: ev.Name, Attribute: ev.Attribute, Type: ev.Type, Builtin: ev.Builtin,
				Canonical: ev.Canonical, Line: ev.Line, Column: ev.Column})
		}
	}
	if s.cfg.SchemaLocationHints {
		opts.Hints = &hintLoader{set: s, dir: path.Dir(name), loaded: append([]string(nil), s.names...)}
	}
	verrs, err := validate.Validate(s.schema, r, opts)
	var out []Error
	if len(verrs) > 0 {
		out = make([]Error, len(verrs))
		for i, e := range verrs {
			out[i] = Error{Code: e.Code, Line: e.Line, Column: e.Column, Path: e.Path, Name: e.Name, Value: e.Value,
				Facet: e.Facet, Expected: e.Expected, Message: e.Message, Namespace: e.Namespace, Reason: e.Reason,
				ExpectedNamespace: e.ExpectedNamespace, Type: e.Type, TypeNamespace: e.TypeNamespace,
				Builtin: e.Builtin, InsertLine: e.InsertLine, InsertColumn: e.InsertColumn}
		}
	}
	if err != nil && !errors.Is(err, ErrTooManyErrors) {
		err = fmt.Errorf("xsd: %w", err)
	}
	return out, err
}

// hintLoader wczytuje dokumenty ze wskazówek jednego dokumentu; schematy są wspólne dla Set (pamięć podręczna).
type hintLoader struct {
	set    *Set
	dir    string
	loaded []string
}

func (h *hintLoader) LoadHints(hints []validate.Hint) (*schema.Schema, error) {
	added := false
	for _, hint := range hints {
		p, ok := h.resolve(hint.Location)
		if !ok {
			continue // wskazówka bez dokumentu w fsys: procesor może ją pominąć (4.3.2)
		}
		dup := false
		for _, q := range h.loaded {
			if q == p {
				dup = true
				break
			}
		}
		if !dup {
			h.loaded = append(h.loaded, p)
			added = true
		}
	}
	if !added {
		return nil, nil
	}
	return h.set.schemaFor(h.loaded)
}

func (h *hintLoader) resolve(loc string) (string, bool) {
	if p, ok := h.set.cfg.Locations[loc]; ok {
		return p, true
	}
	if strings.Contains(loc, ":") || strings.HasPrefix(loc, "/") {
		return "", false
	}
	p := path.Clean(path.Join(h.dir, loc))
	if !fs.ValidPath(p) {
		return "", false
	}
	if _, err := fs.Stat(h.set.fsys, p); err != nil {
		return "", false
	}
	return p, true
}

func (s *Set) schemaFor(names []string) (*schema.Schema, error) {
	key := append([]string(nil), names...)
	sort.Strings(key)
	k := strings.Join(key, "\x00")
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.hinted[k]; ok {
		return h.sch, h.err
	}
	sch, err := schema.Load(s.fsys, s.cfg.schemaConfig(), names...)
	if s.hinted == nil {
		s.hinted = map[string]hintedSchema{}
	}
	s.hinted[k] = hintedSchema{sch, err}
	return sch, err
}

// Error to jeden błąd walidacji dokumentu albo schematu.
type Error struct {
	// Code to stały kod błędu, nazwany według reguły rekomendacji (np. "cvc-complex-type.2.4", "src-resolve").
	Code string
	// Document to ścieżka dokumentu schematu w fsys dla błędów schematu; pusta dla błędów dokumentu.
	Document string
	// Line i Column to pozycja w dokumencie, liczone od 1; kolumna w znakach.
	Line, Column int
	// Path to ścieżka elementu, np. "/JPK/Ewidencja/SprzedazWiersz[3]": nazwy lokalne bez prefiksów, [n] od
	// drugiego wystąpienia nazwy wśród rodzeństwa, "/@nazwa" dla atrybutu (ADR 0008).
	Path string
	// Name to nazwa lokalna elementu albo atrybutu, którego dotyczy błąd.
	Name string
	// Value to wartość, której dotyczy błąd (np. tekst niezgodny z typem, zdublowany klucz).
	Value string
	// Facet to naruszony aspekt i jego wartość, np. "maxLength 5"; puste, gdy błąd nie dotyczy aspektu.
	Facet string
	// Expected to elementy dozwolone w miejscu błędu cvc-complex-type.2.4: "nazwa" albo "{przestrzeń}nazwa",
	// symbol wieloznaczny jako "*" albo "{przestrzeń}*".
	Expected []string
	// Message to komunikat po angielsku; do tłumaczenia służy Code (z parametrami z pól powyżej i niżej).
	Message string

	// Namespace to przestrzeń nazw elementu albo atrybutu Name; pusta dla nazwy bez przestrzeni (ADR 0010).
	Namespace string
	// Reason rozróżnia błędy cvc-complex-type.2.4: ReasonMissing (brak wymaganego elementu), ReasonUnexpected
	// (element niedozwolony w tym miejscu), ReasonWrongNamespace (nazwa lokalna oczekiwanego elementu w innej
	// przestrzeni nazw). Dla innych kodów pusty; kod reguły jest zawsze ten sam.
	Reason string
	// ExpectedNamespace to przestrzeń nazw, w której schemat oczekuje elementu (tylko ReasonWrongNamespace).
	ExpectedNamespace string
	// Type i TypeNamespace to typ ze schematu, któremu nie odpowiada wartość (błędy typów prostych, np. "TData");
	// Type jest pusty dla typu anonimowego. Builtin to najbliższy typ wbudowany, z którego typ pochodzi
	// (np. "date", "decimal").
	Type, TypeNamespace, Builtin string
	// InsertLine i InsertColumn (cvc-complex-type.2.4) wskazują, gdzie powinien zacząć się oczekiwany element:
	// przed niedozwolonym elementem (początek jego znacznika) albo przed znacznikiem końcowym rodzica; dla rodzica
	// zapisanego jako <x/> koniec tego znacznika. Pozycja główna (Line, Column) się nie zmienia (ADR 0008).
	InsertLine, InsertColumn int
}

// Wartości Error.Reason dla cvc-complex-type.2.4 (ADR 0010).
const (
	ReasonMissing        = validate.ReasonMissing
	ReasonUnexpected     = validate.ReasonUnexpected
	ReasonWrongNamespace = validate.ReasonWrongNamespace
)

func (e Error) Error() string {
	if e.Document != "" {
		return fmt.Sprintf("%s:%d:%d: %s: %s", e.Document, e.Line, e.Column, e.Code, e.Message)
	}
	return fmt.Sprintf("%d:%d: %s: %s", e.Line, e.Column, e.Code, e.Message)
}
