package xsd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// fakeWeb to serwer z pamięci dla Config.Resolve: adres → treść; liczy zapytania.
type fakeWeb struct {
	docs  map[string]string
	calls map[string]int
	refs  []SchemaRef
	err   map[string]error
}

func (w *fakeWeb) resolve(r SchemaRef) (io.ReadCloser, error) {
	if w.calls == nil {
		w.calls = map[string]int{}
	}
	w.calls[r.Location]++
	w.refs = append(w.refs, r)
	if err := w.err[r.Location]; err != nil {
		return nil, err
	}
	d, ok := w.docs[r.Location]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(d)), nil
}

const xsdNS = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`

// TestResolveRecursive: dokumenty z Config.Resolve są czytane w tym samym przebiegu co reszta; ich include
// i import (także adresy względne, rozwiązane względem adresu dokumentu) przechodzą przez te same reguły; każdy
// adres pada najwyżej raz, także przy cyklu; Documents podaje każdy dokument z nazwą, adresem, źródłem i SHA-256.
// REQ: SCH-022 SCH-023
func TestResolveRecursive(t *testing.T) {
	web := &fakeWeb{docs: map[string]string{
		"https://example.com/s/a.xsd": `<xs:schema ` + xsdNS + ` targetNamespace="urn:a" xmlns:a="urn:a" xmlns:c="urn:c">
  <xs:include schemaLocation="b.xsd"/>
  <xs:import namespace="urn:c" schemaLocation="../common/c.xsd"/>
  <xs:element name="a" type="a:T"/>
  <xs:complexType name="T"><xs:sequence><xs:element ref="c:c"/><xs:element name="b" type="a:B"/></xs:sequence></xs:complexType>
</xs:schema>`,
		"https://example.com/s/b.xsd": `<xs:schema ` + xsdNS + ` targetNamespace="urn:a">
  <xs:include schemaLocation="a.xsd"/>
  <xs:simpleType name="B"><xs:restriction base="xs:string"><xs:maxLength value="3"/></xs:restriction></xs:simpleType>
</xs:schema>`,
		"https://example.com/common/c.xsd": `<xs:schema ` + xsdNS + ` targetNamespace="urn:c">
  <xs:element name="c" type="xs:int"/>
</xs:schema>`,
	}}
	fsys := fstest.MapFS{"main.xsd": {Data: []byte(`<xs:schema ` + xsdNS + ` targetNamespace="urn:m" xmlns:a="urn:a">
  <xs:import namespace="urn:a" schemaLocation="https://example.com/s/a.xsd"/>
  <xs:element name="m"><xs:complexType><xs:sequence><xs:element ref="a:a"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)}}
	set, err := Config{Resolve: web.resolve}.Compile(fsys, "main.xsd")
	if err != nil {
		t.Fatal(err)
	}
	for u, n := range web.calls {
		if n != 1 {
			t.Errorf("%s asked %d times", u, n)
		}
	}
	if len(web.calls) != 3 {
		t.Errorf("asked for %v", web.calls)
	}
	if r := web.refs[0]; r.Kind != "import" || r.Namespace != "urn:a" || r.From != "main.xsd" {
		t.Errorf("first ref %+v", r)
	}
	docs := set.Documents()
	var names []string
	for _, d := range docs {
		names = append(names, d.Name)
		want := d.Name != "main.xsd"
		if d.Resolved != want || (want && d.Location != d.Name) {
			t.Errorf("%+v", d)
		}
		data := web.docs[d.Name]
		if d.Name == "main.xsd" {
			data = string(fsys["main.xsd"].Data)
		}
		sum := sha256.Sum256([]byte(data))
		if d.SHA256 != hex.EncodeToString(sum[:]) || d.Size != int64(len(data)) {
			t.Errorf("%s: size %d sha %s", d.Name, d.Size, d.SHA256)
		}
	}
	if got := strings.Join(names, " "); got != "main.xsd https://example.com/s/a.xsd https://example.com/s/b.xsd https://example.com/common/c.xsd" {
		t.Errorf("documents %s", got)
	}
	ok := `<m:m xmlns:m="urn:m" xmlns:a="urn:a" xmlns:c="urn:c"><a:a><c:c>1</c:c><b>abc</b></a:a></m:m>`
	if errs, err := set.Validate(strings.NewReader(ok)); err != nil || len(errs) != 0 {
		t.Errorf("valid document: %v %v", errs, err)
	}
	bad := strings.Replace(ok, "abc", "abcd", 1)
	if errs, _ := set.Validate(strings.NewReader(bad)); len(errs) == 0 || errs[0].Code != "cvc-maxLength-valid" {
		t.Errorf("invalid document: %v", errs)
	}
}

// TestResolveOrder: Locations i ścieżka w fs.FS mają pierwszeństwo przed Resolve, a dla importu także
// Namespaces; Resolve nie jest pytany o to, co biblioteka znajduje sama. Bez Resolve adres bezwzględny zostaje
// nierozwiązany jak dotąd.
// REQ: SCH-022
func TestResolveOrder(t *testing.T) {
	c := `<xs:schema ` + xsdNS + ` targetNamespace="urn:c"><xs:element name="c" type="xs:int"/></xs:schema>`
	fsys := fstest.MapFS{
		"main.xsd": {Data: []byte(`<xs:schema ` + xsdNS + ` targetNamespace="urn:m">
  <xs:import namespace="urn:c" schemaLocation="http://example.com/c.xsd"/>
  <xs:import namespace="urn:d" schemaLocation="http://example.com/d.xsd"/>
  <xs:include schemaLocation="local.xsd"/>
</xs:schema>`)},
		"local.xsd":    {Data: []byte(`<xs:schema ` + xsdNS + ` targetNamespace="urn:m"/>`)},
		"mirror/c.xsd": {Data: []byte(c)},
		"d.xsd":        {Data: []byte(`<xs:schema ` + xsdNS + ` targetNamespace="urn:d"/>`)},
	}
	web := &fakeWeb{docs: map[string]string{"http://example.com/c.xsd": c}}
	set, err := Config{Locations: map[string]string{"http://example.com/c.xsd": "mirror/c.xsd"},
		Namespaces: map[string]string{"urn:d": "d.xsd"}, Resolve: web.resolve}.Compile(fsys, "main.xsd")
	if err != nil {
		t.Fatal(err)
	}
	if len(web.calls) != 0 {
		t.Errorf("Resolve asked for %v", web.calls)
	}
	if n := len(set.Documents()); n != 4 {
		t.Errorf("%d documents", n)
	}
	// Bez wpisów: Resolve dostaje oba importy.
	web = &fakeWeb{docs: map[string]string{"http://example.com/c.xsd": c}}
	if _, err := (Config{Resolve: web.resolve}).Compile(fsys, "main.xsd"); err != nil {
		t.Fatal(err)
	}
	if web.calls["http://example.com/c.xsd"] != 1 || web.calls["http://example.com/d.xsd"] != 1 {
		t.Errorf("calls %v", web.calls)
	}
}

// TestResolveErrors: fs.ErrNotExist z Resolve znaczy „nie ma” (nierozwiązany include nie jest błędem, odwołanie
// do brakującego komponentu już tak); inny błąd przerywa Compile i jest opakowany; dokument większy niż
// MaxResolvedSize jest odrzucany; błąd schematu w dokumencie z Resolve wskazuje jego adres; MaxDocuments
// ogranicza też zapytania.
// REQ: SCH-022
func TestResolveErrors(t *testing.T) {
	main := fstest.MapFS{"main.xsd": {Data: []byte(`<xs:schema ` + xsdNS + `>
  <xs:include schemaLocation="https://example.com/x.xsd"/>
</xs:schema>`)}}

	web := &fakeWeb{}
	if _, err := (Config{Resolve: web.resolve}).Compile(main, "main.xsd"); err != nil {
		t.Errorf("not found: %v", err)
	}

	boom := errors.New("connection refused")
	web = &fakeWeb{err: map[string]error{"https://example.com/x.xsd": boom}}
	_, err := Config{Resolve: web.resolve}.Compile(main, "main.xsd")
	var se *SchemaError
	if !errors.Is(err, boom) || errors.As(err, &se) || !strings.Contains(err.Error(), "https://example.com/x.xsd") {
		t.Errorf("resolver error: %v", err)
	}

	big := func(r SchemaRef) (io.ReadCloser, error) {
		return io.NopCloser(io.LimitReader(zeros{}, MaxResolvedSize+1)), nil
	}
	if _, err := (Config{Resolve: big}).Compile(main, "main.xsd"); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("large document: %v", err)
	}

	web = &fakeWeb{docs: map[string]string{"https://example.com/x.xsd": `<xs:schema ` + xsdNS + `>
  <xs:element name="e" type="undefined"/>
</xs:schema>`}}
	_, err = Config{Resolve: web.resolve}.Compile(main, "main.xsd")
	if !errors.As(err, &se) || se.Errors[0].Document != "https://example.com/x.xsd" || se.Errors[0].Line != 2 {
		t.Errorf("schema error in a resolved document: %v", err)
	}

	// Łańcuch dokumentów z Resolve: limit dokumentów obowiązuje i ogranicza zapytania.
	chain := func(r SchemaRef) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`<xs:schema ` + xsdNS + `><xs:include schemaLocation="` +
			r.Location + `x"/></xs:schema>`)), nil
	}
	calls := 0
	counted := func(r SchemaRef) (io.ReadCloser, error) { calls++; return chain(r) }
	_, err = Config{Resolve: counted, MaxDocuments: 5}.Compile(main, "main.xsd")
	if !errors.As(err, &se) || se.Errors[0].Code != "schema-limit" || calls > 5 {
		t.Errorf("chain: %d calls, %v", calls, err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}
