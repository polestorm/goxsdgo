package xsd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/polestorm/goxsdgo/internal/testkit"
)

// Generator syntetycznego JPK_V7M(3): nagłówek i stopka z przykładu MF (dane fikcyjne MF), wiersz sprzedaży
// powielany z kolejnym numerem i wymyślonym numerem kontrahenta. Strumień jest liczony w locie, bez pliku.

const jpkSample = "../testdata/instances/mf/jpk_v7m_3_organization.xml"

type jpkParts struct{ head, row, tail []byte }

func loadJPKParts(t testing.TB) jpkParts {
	t.Helper()
	b, err := os.ReadFile(jpkSample)
	if err != nil {
		t.Skip(testkit.FetchHint)
	}
	s := string(b)
	i := strings.Index(s, "<SprzedazWiersz>")
	j := strings.Index(s, "</SprzedazWiersz>") + len("</SprzedazWiersz>")
	if i < 0 || j < i {
		t.Fatal("sample has no SprzedazWiersz")
	}
	return jpkParts{head: []byte(s[:i]), row: []byte(s[i:j]), tail: []byte(s[j:])}
}

// jpkReader podaje dokument o rozmiarze co najmniej size bajtów.
type jpkReader struct {
	p    jpkParts
	size int64
	n    int64 // wygenerowane bajty
	row  int
	buf  []byte
	done bool
}

func newJPKReader(p jpkParts, size int64) *jpkReader {
	r := &jpkReader{p: p, size: size}
	r.buf = append(r.buf, p.head...)
	return r
}

func (r *jpkReader) Read(b []byte) (int, error) {
	for len(r.buf) == 0 {
		if r.done {
			return 0, io.EOF
		}
		if r.n >= r.size {
			r.buf = append(r.buf, r.p.tail...)
			r.done = true
			break
		}
		r.row++
		row := bytes.Replace(r.p.row, []byte("<LpSprzedazy>1</LpSprzedazy>"),
			[]byte("<LpSprzedazy>"+strconv.Itoa(r.row)+"</LpSprzedazy>"), 1)
		row = bytes.Replace(row, []byte("<NrKontrahenta>a</NrKontrahenta>"),
			[]byte("<NrKontrahenta>K"+strconv.Itoa(r.row%100000)+"</NrKontrahenta>"), 1)
		r.buf = append(r.buf, row...)
	}
	k := copy(b, r.buf)
	r.buf = r.buf[k:]
	if len(r.buf) == 0 {
		r.buf = r.buf[:0:0]
	}
	r.n += int64(k)
	return k, nil
}

func compileJPK(t testing.TB) *Set {
	t.Helper()
	src, err := testkit.LoadSources(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(schemasDir + "/jpk_v7m_3.xsd"); err != nil {
		t.Skip(testkit.FetchHint)
	}
	set, err := Config{Locations: src.Locations(schemasSetName)}.Compile(os.DirFS(schemasDir), "jpk_v7m_3.xsd")
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// TestStreamingMemory mierzy kryterium K6: szczyt pamięci przy walidacji dużego JPK nie zależy od rozmiaru
// pliku. Domyślnie 200 MB (20 MB z -short); GOXSDGO_STREAM_MB=1024 sprawdza plik 1 GB. REQ: VAL-018
func TestStreamingMemory(t *testing.T) {
	set := compileJPK(t)
	p := loadJPKParts(t)
	mb := int64(200)
	if testing.Short() {
		mb = 20
	}
	if s := os.Getenv("GOXSDGO_STREAM_MB"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		mb = n
	}
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak uint64
	stopc := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if u := m.HeapInuse + m.StackInuse; u > peak {
				peak = u
			}
			select {
			case <-stopc:
				return
			case <-tick.C:
			}
		}
	}()
	start := time.Now()
	r := newJPKReader(p, mb<<20)
	errs, err := set.Validate(r)
	elapsed := time.Since(start)
	close(stopc)
	wg.Wait()
	if err != nil || len(errs) != 0 {
		t.Fatalf("synthetic JPK is not valid: %v %v", err, errs[:min(3, len(errs))])
	}
	t.Logf("validated %d MB (%d rows) in %v (%.0f MB/s); peak heap+stack in use %.1f MB (before: %.1f MB)",
		r.n>>20, r.row, elapsed.Round(time.Millisecond), float64(r.n)/(1<<20)/elapsed.Seconds(),
		float64(peak)/(1<<20), float64(base.HeapInuse+base.StackInuse)/(1<<20))
	if peak > 64<<20 {
		t.Errorf("peak memory %.1f MB exceeds 64 MB", float64(peak)/(1<<20))
	}
}

// BenchmarkValidateJPK mierzy szybkość walidacji (K7) na syntetycznym JPK_V7M(3) z ok. 20 MB w pamięci.
// GOXSDGO_WRITE_JPK=ścieżka zapisuje ten sam dokument do pliku (do porównania z xmllint --stream).
func BenchmarkValidateJPK(b *testing.B) {
	set := compileJPK(b)
	p := loadJPKParts(b)
	doc, err := io.ReadAll(newJPKReader(p, 20<<20))
	if err != nil {
		b.Fatal(err)
	}
	if out := os.Getenv("GOXSDGO_WRITE_JPK"); out != "" {
		mb, _ := strconv.Atoi(os.Getenv("GOXSDGO_WRITE_JPK_MB"))
		data := doc
		if mb > 0 {
			data, _ = io.ReadAll(newJPKReader(p, int64(mb)<<20))
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errs, err := set.Validate(bytes.NewReader(doc))
		if err != nil || len(errs) != 0 {
			b.Fatal(err, errs)
		}
	}
}

// TestDeterministic: wielokrotna i współbieżna walidacja tego samego dokumentu daje te same błędy w tej samej
// kolejności (powtarzalność, ADR 0001 p. 7). REQ: VAL-019
func TestDeterministic(t *testing.T) {
	set := compileJPK(t)
	p := loadJPKParts(t)
	// Dokument z wieloma błędami różnych rodzajów: zła data, nieznany element, zły typ, zła wartość wyliczenia.
	var b bytes.Buffer
	b.Write(p.head)
	for i := 0; i < 50; i++ {
		row := string(p.row)
		switch i % 4 {
		case 0:
			row = strings.Replace(row, "2026-02-01", "2026-13-01", 1)
		case 1:
			row = strings.Replace(row, "<GTU_01>1</GTU_01>", "<GTU_99>1</GTU_99>", 1)
		case 2:
			row = strings.Replace(row, "<K_10>0</K_10>", "<K_10>zero</K_10>", 1)
		case 3:
			row = strings.Replace(row, "<TypDokumentu>FP</TypDokumentu>", "<TypDokumentu>XX</TypDokumentu>", 1)
		}
		b.WriteString(row)
	}
	b.Write(p.tail)
	doc := b.Bytes()
	render := func(errs []Error) string {
		var s strings.Builder
		for _, e := range errs {
			fmt.Fprintf(&s, "%s|%d|%d|%s|%s|%s|%v\n", e.Code, e.Line, e.Column, e.Path, e.Value, e.Facet, e.Expected)
		}
		return s.String()
	}
	first, err := set.Validate(bytes.NewReader(doc))
	if err != nil || len(first) < 50 {
		t.Fatalf("expected many errors: %v, %d", err, len(first))
	}
	want := render(first)
	var wg sync.WaitGroup
	results := make([]string, 16)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs, _ := set.Validate(bytes.NewReader(doc))
			results[i] = render(errs)
		}(i)
	}
	wg.Wait()
	for i, r := range results {
		if r != want {
			t.Fatalf("run %d differs from the first run", i)
		}
	}
}

// openRecorder zapisuje każde otwarcie pliku w fs.
type openRecorder struct {
	fs.FS
	mu    sync.Mutex
	names []string
}

func (o *openRecorder) Open(name string) (fs.File, error) {
	o.mu.Lock()
	o.names = append(o.names, name)
	o.mu.Unlock()
	return o.FS.Open(name)
}

// TestSafeByDefault (K6): bez sieci i encji zewnętrznych, wskazówki xsi:schemaLocation wyłączone, limity
// głębokości. REQ: VAL-020
func TestSafeByDefault(t *testing.T) {
	rec := &openRecorder{FS: fstest.MapFS{
		"s.xsd": {Data: []byte(`<xs:schema ` + testNS + `><xs:element name="r" type="xs:anyType"/></xs:schema>`)},
		"x.xsd": {Data: []byte(`<xs:schema ` + testNS + `/>`)},
	}}
	set, err := Compile(rec, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	opened := len(rec.names)

	// Encja zewnętrzna nigdy nie jest czytana: walidacja kończy się errors.ErrUnsupported.
	_, err = set.Validate(strings.NewReader(`<!DOCTYPE r [<!ENTITY e SYSTEM "http://127.0.0.1:9/x">]><r>&e;</r>`))
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("external entity: %v", err)
	}
	// Wskazówki są pomijane przy ustawieniach domyślnych: żaden dokument nie jest otwierany.
	errs, err := set.Validate(strings.NewReader(`<r xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
		xsi:noNamespaceSchemaLocation="x.xsd" xsi:schemaLocation="urn:z http://127.0.0.1:9/z.xsd"/>`))
	if err != nil || len(errs) != 0 {
		t.Errorf("hints: %v %v", err, errs)
	}
	if len(rec.names) != opened {
		t.Errorf("validation opened files: %v", rec.names[opened:])
	}
	// Z włączonymi wskazówkami adres w sieci bez wpisu w Locations też nie jest otwierany.
	hinted, err := Config{SchemaLocationHints: true}.Compile(rec, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	opened = len(rec.names)
	if _, err := hinted.Validate(strings.NewReader(`<r xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
		xsi:schemaLocation="urn:z http://127.0.0.1:9/z.xsd"/>`)); err != nil {
		t.Errorf("network hint: %v", err)
	}
	for _, n := range rec.names[opened:] {
		if strings.Contains(n, "127.0.0.1") {
			t.Errorf("opened %s", n)
		}
	}
	// Limit głębokości czytnika: ograniczenie biblioteki (ErrLimit), nie werdykt o dokumencie.
	deep := strings.Repeat("<r>", 600) + strings.Repeat("</r>", 600)
	if _, err := set.Validate(strings.NewReader(deep)); !errors.Is(err, ErrLimit) {
		t.Errorf("depth limit: %v", err)
	}
	// Limit wpisów ID: tysiąc różnych ID przy limicie 100.
	ids, err := Config{MaxIdentityEntries: 100}.Compile(fstest.MapFS{"s.xsd": {Data: []byte(`<xs:schema ` + testNS + `>
		<xs:element name="r"><xs:complexType><xs:sequence><xs:element name="i" maxOccurs="unbounded"><xs:complexType>
		<xs:attribute name="id" type="xs:ID"/></xs:complexType></xs:element></xs:sequence></xs:complexType></xs:element></xs:schema>`)}}, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	var many strings.Builder
	many.WriteString("<r>")
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&many, `<i id="x%d"/>`, i)
	}
	many.WriteString("</r>")
	if _, err := ids.Validate(strings.NewReader(many.String())); !errors.Is(err, ErrLimit) {
		t.Errorf("identity limit: %v", err)
	}
}
