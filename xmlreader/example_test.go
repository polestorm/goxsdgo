package xmlreader_test

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/polestorm/goxsdgo/xmlreader"
)

func Example() {
	doc := `<inv:Invoice xmlns:inv="urn:example:invoice" no="FV/1">
  <inv:Total currency="PLN">123.45</inv:Total>
</inv:Invoice>`
	r := xmlreader.New(strings.NewReader(doc), xmlreader.Options{})
	for {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		switch t.Kind {
		case xmlreader.KindStartElement:
			fmt.Printf("%d:%d start {%s}%s", t.Line, t.Column, t.Name.Space, t.Name.Local)
			for _, a := range t.Attrs {
				fmt.Printf(" %s=%q", a.Name.Local, a.Value)
			}
			fmt.Println()
		case xmlreader.KindEndElement:
			fmt.Printf("%d:%d end %s\n", t.Line, t.Column, t.Name.Local)
		case xmlreader.KindText:
			if !t.Whitespace {
				fmt.Printf("%d:%d text %q\n", t.Line, t.Column, t.Data)
			}
		}
	}
	// Output:
	// 1:1 start {urn:example:invoice}Invoice no="FV/1"
	// 2:3 start {urn:example:invoice}Total currency="PLN"
	// 2:29 text "123.45"
	// 2:35 end Total
	// 3:1 end Invoice
}

func Example_error() {
	r := xmlreader.New(strings.NewReader("<a>\n  <b></a>"), xmlreader.Options{})
	for {
		_, err := r.Next()
		var e *xmlreader.Error
		if errors.As(err, &e) {
			fmt.Println(e.Line, e.Column, e.Code)
			return
		}
		if err != nil {
			return
		}
	}
	// Output:
	// 2 6 wfc-element-type-match
}
