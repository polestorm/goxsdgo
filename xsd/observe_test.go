package xsd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// codeObserver zbiera kody błędów zwrócone w testach zgodności (W3C, korpus): rodzaj (schemat albo dokument),
// pola ustawione w Error i przykład komunikatu. Służy do sprawdzenia katalogu kodów (catalog_test.go): każdy kod
// zaobserwowany w zwróconym błędzie jest w katalogu, z rodzajem i parametrami, które katalog deklaruje.
type codeObserver struct {
	mu    sync.Mutex
	codes map[string]*observedCode
}

type observedCode struct {
	kinds   map[string]bool
	fields  map[string]bool
	count   int
	example string
}

func newCodeObserver() *codeObserver { return &codeObserver{codes: map[string]*observedCode{}} }

func (o *codeObserver) add(kind string, errs []Error) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, e := range errs {
		c := o.codes[e.Code]
		if c == nil {
			c = &observedCode{kinds: map[string]bool{}, fields: map[string]bool{}, example: e.Message}
			o.codes[e.Code] = c
		}
		c.count++
		c.kinds[kind] = true
		if e.Namespace != "" && e.Name == "" {
			c.fields["Namespace without Name"] = true
		}
		for name, set := range map[string]bool{"Name": e.Name != "", "Value": e.Value != "", "Facet": e.Facet != "",
			"Expected": len(e.Expected) > 0, "Path": e.Path != "", "Document": e.Document != "",
			"Reason": e.Reason != "" || e.ExpectedNamespace != "", "Type": e.Type != "" || e.TypeNamespace != "" || e.Builtin != "",
			"Insert": e.InsertLine != 0 || e.InsertColumn != 0} {
			if set {
				c.fields[name] = true
			}
		}
	}
}

func (o *codeObserver) schemaErr(err error) {
	if se, ok := err.(*SchemaError); ok {
		o.add(KindSchema, se.Errors)
	}
}

// dump zapisuje obserwacje do pliku TSV (zmienna GOXSDGO_CODE_DUMP); pomaga utrzymać katalog.
func (o *codeObserver) dump(path string) error {
	keys := make([]string, 0, len(o.codes))
	for k := range o.codes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		c := o.codes[k]
		fmt.Fprintf(&b, "%s\t%s\t%s\t%d\t%s\n", k, setString(c.kinds), setString(c.fields), c.count, oneLine(c.example))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func setString(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
