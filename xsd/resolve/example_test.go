package resolve_test

import (
	"fmt"
	"strings"
	"testing/fstest"

	"github.com/polestorm/goxsdgo/xsd"
	"github.com/polestorm/goxsdgo/xsd/resolve"
)

func Example() {
	// Schemas the program ships, laid out as host/path (an embed.FS works the same way).
	shipped := fstest.MapFS{
		"example.com/types.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:string"><xs:length value="3"/></xs:restriction></xs:simpleType>
</xs:schema>`)},
	}
	own := fstest.MapFS{
		"profile.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="https://example.com/types.xsd"/>
  <xs:element name="code" type="Code"/>
</xs:schema>`)},
	}
	set, err := xsd.Config{Resolve: resolve.New(resolve.Options{Dir: shipped})}.Compile(own, "profile.xsd")
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, d := range set.Documents() {
		fmt.Println(d.Name, d.Resolved, d.SHA256[:12])
	}
	errs, _ := set.Validate(strings.NewReader("<code>ABCD</code>"))
	fmt.Println(errs[0].Code)
	// Output:
	// profile.xsd false b2d7db13851a
	// https://example.com/types.xsd true c1de97c5ee19
	// cvc-length-valid
}
