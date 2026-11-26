package xsd

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"testing/iotest"
)

// TestValidateChunkedInput: walidacja daje ten sam wynik, gdy dokument przychodzi po jednym bajcie (np. z sieci
// małymi porcjami), jak przy jednym odczycie. Do v0.4.0 deklaracja XML czytana porcjami kończyła się paniką
// czytnika (docs/known-issues.md, KI-13).
// REQ: XML-019
func TestValidateChunkedInput(t *testing.T) {
	fsys := fstest.MapFS{"s.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a"><xs:complexType><xs:attribute name="n" type="xs:int"/></xs:complexType></xs:element>
</xs:schema>`)}}
	set, err := Compile(fsys, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range []string{`<?xml version="1.0"?><a n="1"/>`, `<?xml version="1.0" encoding="UTF-8" ?><a n="x"/>`,
		"<?xml version='1.0'\n\t?>\n<a n='2'/>"} {
		want, werr := set.Validate(strings.NewReader(doc))
		got, gerr := set.Validate(iotest.OneByteReader(strings.NewReader(doc)))
		if werr != nil || gerr != nil || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q: one byte at a time %v %v, whole %v %v", doc, got, gerr, want, werr)
		}
	}
}
