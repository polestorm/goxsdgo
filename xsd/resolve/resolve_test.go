package resolve

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/polestorm/goxsdgo/xsd"
)

const xs = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`

func ref(loc string) xsd.SchemaRef {
	return xsd.SchemaRef{Location: loc, Kind: "include", From: "main.xsd"}
}

func read(t *testing.T, f func(xsd.SchemaRef) (io.ReadCloser, error), loc string) string {
	t.Helper()
	rc, err := f(ref(loc))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestLocalPath: układ host/ścieżka; adresy z zapytaniem, bez nazwy pliku albo wychodzące poza katalog hosta nie
// mają ścieżki.
// REQ: SCH-024
func TestLocalPath(t *testing.T) {
	for in, want := range map[string]string{
		"http://crd.gov.pl/wzor/2025/12/19/14090/schemat.xsd": "crd.gov.pl/wzor/2025/12/19/14090/schemat.xsd",
		"https://WWW.W3.ORG/2001/xml.xsd":                     "www.w3.org/2001/xml.xsd",
		"http://localhost:8080/a/../b.xsd":                    "localhost_8080/b.xsd",
		"http://example.com/../../etc/passwd":                 "example.com/etc/passwd",
		"http://example.com/a.xsd?x=1":                        "",
		"http://example.com/":                                 "",
		"http://example.com/dir/":                             "",
	} {
		u, _ := url.Parse(in)
		got, ok := LocalPath(u)
		if got != want || ok != (want != "") {
			t.Errorf("%s: %q %v, want %q", in, got, ok, want)
		}
	}
}

// TestDirCacheNetwork: najpierw Dir, potem Cache; sieć tylko z Network i tylko do hostów z AllowHosts; pobrany
// dokument trafia do Cache i następnym razem jest czytany z dysku; błąd pobierania przerywa kompilację (ErrDownload),
// a brak dokumentu przy wyłączonej sieci znaczy „nie ma”.
// REQ: SCH-024
func TestDirCacheNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/s/a.xsd":
			io.WriteString(w, `<xs:schema `+xs+`><xs:include schemaLocation="b.xsd"/><xs:element name="a" type="B"/></xs:schema>`)
		case "/s/b.xsd":
			io.WriteString(w, `<xs:schema `+xs+`><xs:simpleType name="B"><xs:restriction base="xs:int"/></xs:simpleType></xs:schema>`)
		case "/away":
			http.Redirect(w, r, "http://elsewhere.invalid/x.xsd", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	base := srv.URL + "/s/"

	dir := fstest.MapFS{"example.com/d.xsd": {Data: []byte("from dir")}}
	cache := t.TempDir()
	os.MkdirAll(filepath.Join(cache, "example.com"), 0o755)
	os.WriteFile(filepath.Join(cache, "example.com", "d.xsd"), []byte("from cache"), 0o644)
	os.WriteFile(filepath.Join(cache, "example.com", "c.xsd"), []byte("cached c"), 0o644)

	off := New(Options{Dir: dir, Cache: cache})
	if s := read(t, off, "http://example.com/d.xsd"); s != "from dir" {
		t.Errorf("Dir first: %q", s)
	}
	if s := read(t, off, "https://example.com/c.xsd"); s != "cached c" {
		t.Errorf("Cache: %q", s)
	}
	if _, err := off(ref(base + "a.xsd")); !errors.Is(err, fs.ErrNotExist) || hits.Load() != 0 {
		t.Errorf("network off: %v, %d requests", err, hits.Load())
	}
	if _, err := New(Options{Network: true, AllowHosts: []string{"crd.gov.pl"}})(ref(base + "a.xsd")); !errors.Is(err, fs.ErrNotExist) || hits.Load() != 0 {
		t.Errorf("host not allowed: %v", err)
	}

	on := New(Options{Cache: cache, Network: true, AllowHosts: []string{host}})
	main := fstest.MapFS{"main.xsd": {Data: []byte(`<xs:schema ` + xs + `><xs:include schemaLocation="` + base + `a.xsd"/></xs:schema>`)}}
	set, err := xsd.Config{Resolve: on}.Compile(main, "main.xsd")
	if err != nil {
		t.Fatal(err)
	}
	if errs, _ := set.Validate(strings.NewReader("<a>x</a>")); len(errs) != 1 {
		t.Errorf("schema from the network not used: %v", errs)
	}
	if hits.Load() != 2 {
		t.Errorf("%d requests, want 2", hits.Load())
	}
	p, _ := LocalPath(u.JoinPath("s", "b.xsd"))
	if _, err := os.Stat(filepath.Join(cache, filepath.FromSlash(p))); err != nil {
		t.Errorf("not cached: %v", err)
	}
	if _, err := (xsd.Config{Resolve: on}).Compile(main, "main.xsd"); err != nil || hits.Load() != 2 {
		t.Errorf("second compilation: %v, %d requests", err, hits.Load())
	}

	for _, loc := range []string{base + "missing.xsd", srv.URL + "/away"} {
		_, err := on(ref(loc))
		if !errors.Is(err, ErrDownload) {
			t.Errorf("%s: %v", loc, err)
		}
	}
	small := New(Options{Network: true, MaxSize: 10})
	if _, err := small(ref(base + "a.xsd")); !errors.Is(err, ErrDownload) || !strings.Contains(err.Error(), "larger") {
		t.Errorf("MaxSize: %v", err)
	}
	if _, err := on(ref("file:///etc/passwd")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("file URL: %v", err)
	}
}
