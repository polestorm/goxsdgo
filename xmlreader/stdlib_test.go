package xmlreader

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
)

// stdlibOut: plik TSV z wynikiem obu czytników dla każdego przypadku XML Conformance (docs/results).
var stdlibOut = flag.String("stdlib-out", "", "write per-case results of xmlreader and encoding/xml to this TSV file")

// stdDecoder: encoding/xml jako punkt odniesienia (ustawienia domyślne, Strict = true).
func stdDecoder(r io.Reader) *xml.Decoder { return xml.NewDecoder(r) }

// stdlibAccepts czyta cały plik przez encoding/xml (Token, czyli z przestrzeniami nazw) i mówi, czy go przyjął.
func stdlibAccepts(file string) bool {
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()
	d := stdDecoder(f)
	for {
		_, err := d.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
	}
}

// TestStdlibConformance mierzy encoding/xml na tych samych przypadkach W3C XML Conformance co TestXMLConformance,
// dla porównania w xmlreader/README.md. Przypadek dobrze uformowany ("valid", "invalid") zalicza czytnik, który
// przeczyta plik bez błędu (xmlreader dodatkowo z tą samą postacią kanoniczną, jeśli zestaw ją podaje, jak
// w TestXMLConformance); przypadek "not-wf" zalicza czytnik, który plik odrzuci. Test niczego nie wymaga od
// encoding/xml; z flagą -stdlib-out zapisuje wynik każdego przypadku.
func TestStdlibConformance(t *testing.T) {
	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err != nil {
		t.Skip()
	}
	type count struct{ in, ours, std int }
	counts := map[string]*count{}
	var rows []string
	for _, ct := range loadConformance(t) {
		if ct.excluded != "" {
			continue
		}
		kind := "well-formed"
		if ct.Type == "not-wf" {
			kind = "not-wf"
		}
		c := counts[kind]
		if c == nil {
			c = &count{}
			counts[kind] = c
		}
		c.in++
		ours, _ := runConformance(ct)
		std := stdlibAccepts(path.Join(xmlconfRoot, ct.dir, ct.URI)) == (kind == "well-formed")
		if ours {
			c.ours++
		}
		if std {
			c.std++
		}
		rows = append(rows, fmt.Sprintf("%s/%s\t%s\t%s\t%s", ct.Group, ct.ID, kind, pass(ours), pass(std)))
	}
	var b strings.Builder
	total := count{}
	for _, k := range []string{"well-formed", "not-wf"} {
		c := counts[k]
		fmt.Fprintf(&b, "%-12s %4d cases: xmlreader %4d, encoding/xml %4d\n", k, c.in, c.ours, c.std)
		total.in += c.in
		total.ours += c.ours
		total.std += c.std
	}
	fmt.Fprintf(&b, "%-12s %4d cases: xmlreader %4d, encoding/xml %4d\n", "all", total.in, total.ours, total.std)
	t.Log("\n" + b.String())
	if *stdlibOut != "" {
		sort.Strings(rows)
		out := "# W3C XML Conformance, in-scope cases: go test ./xmlreader -run TestStdlibConformance -stdlib-out FILE\n" +
			"# " + strings.ReplaceAll(strings.TrimSpace(b.String()), "\n", "\n# ") + "\n" +
			"case\tkind\txmlreader\tencoding/xml\n" + strings.Join(rows, "\n") + "\n"
		if err := os.WriteFile(*stdlibOut, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func pass(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

// BenchmarkJPKStdlibRaw: encoding/xml bez rozwiązywania przestrzeni nazw (RawToken); najmniej pracy, jaką
// encoding/xml potrafi wykonać na tym dokumencie.
func BenchmarkJPKStdlibRaw(b *testing.B) {
	const records = 20000
	n, _ := io.Copy(io.Discard, jpkLike(records))
	b.SetBytes(n)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d := stdDecoder(jpkLike(records))
		for {
			_, err := d.RawToken()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
