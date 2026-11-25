package main

// Próby czarnej skrzynki na małych wejściach, każdy kandydat z USTAWIENIAMI DOMYŚLNYMI (bez resolverów
// i opcji, które harness W3C dokłada): sieć, encje zewnętrzne, „billion laughs”, głębokie zagnieżdżenie,
// wzorzec w czasie liniowym, wybrane konstrukcje XSD i postać błędu.

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	cbsigma "github.com/cB-Sigma/xsd"
	jacoelho "github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
	"github.com/knroy/go-xml/v2/xdm"
	knroy "github.com/knroy/go-xml/v2/xsd"

	"github.com/polestorm/goxsdgo/xsd"
)

// defaultRun kompiluje dir/s.xsd i waliduje dir/d.xml ustawieniami domyślnymi kandydata.
func defaultRun(name, dir string) string {
	schema, doc := filepath.Join(dir, "s.xsd"), filepath.Join(dir, "d.xml")
	switch name {
	case "goxsdgo":
		set, err := xsd.Compile(os.DirFS(dir), "s.xsd")
		if err != nil {
			return "schema error: " + oneLine(err.Error())
		}
		f, _ := os.Open(doc)
		defer f.Close()
		errs, err := set.Validate(f)
		switch {
		case err != nil:
			return "error: " + oneLine(err.Error())
		case len(errs) > 0:
			return "invalid: " + oneLine(errs[0].Error())
		}
		return "valid"
	case "knroy/go-xml":
		s, err := knroy.LoadFile(schema, knroy.Options{})
		if err != nil {
			return "schema error: " + oneLine(err.Error())
		}
		f, _ := os.Open(doc)
		defer f.Close()
		tree, err := knroy.ParseDocument(f, xdm.ParseOptions{})
		if err != nil {
			return "parse error: " + oneLine(err.Error())
		}
		if err := s.Validate(tree.Root, knroy.ValidateOptions{}); err != nil {
			return "invalid: " + oneLine(err.Error())
		}
		return "valid"
	case "jacoelho/xsd":
		e, err := jacoelho.Compile(jacoelho.File(schema))
		if err != nil {
			return "schema error: " + oneLine(err.Error())
		}
		f, _ := os.Open(doc)
		defer f.Close()
		if err := e.Validate(f); err != nil {
			if xsderrors.IsConclusiveValidation(err) {
				return "invalid: " + oneLine(err.Error())
			}
			return "error: " + oneLine(err.Error())
		}
		return "valid"
	case "cB-Sigma/xsd":
		s, err := cbsigma.Load(os.DirFS(dir), "s.xsd")
		if err != nil {
			return "schema error: " + oneLine(err.Error())
		}
		b, _ := os.ReadFile(doc)
		if err := s.Validate(b); err != nil {
			return "invalid: " + oneLine(err.Error())
		}
		return "valid"
	case "libxml2":
		cmd := exec.Command(xmllint, "--noout", "--schema", schema, doc)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "invalid: " + oneLine(firstLine(stderr.String()))
		}
		return "valid"
	}
	return "?"
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

type probe struct {
	name, schema, doc string
	setup             func(dir string) // dodatkowe pliki
}

func runProbes(root, out string, cands []candidate) error {
	names := []string{}
	for _, c := range cands {
		names = append(names, c.name())
	}
	names = append(names, "libxml2")

	// Serwer HTTP liczący żądania: czy kandydat sięga po schemaLocation do sieci.
	var hits atomic.Int64
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:n"><xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType></xs:schema>`))
	}))
	defer ln.Close()
	url := "http://" + ln.Addr().String() + "/n.xsd"
	secret := filepath.Join(os.TempDir(), "goxsdgo-probe-secret.txt")
	os.WriteFile(secret, []byte("SECRET"), 0o644)
	defer os.Remove(secret)

	const xs = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`
	str := `<xs:schema ` + xs + `><xs:element name="r"><xs:simpleType><xs:restriction base="xs:string"><xs:enumeration value="SECRET"/><xs:enumeration value="ok"/></xs:restriction></xs:simpleType></xs:element></xs:schema>`
	laughs := `<?xml version="1.0"?><!DOCTYPE r [<!ENTITY a "lollollollollollollollollollol">`
	prev := "a"
	for i, n := range "bcdefghi" {
		_ = i
		cur := string(n)
		laughs += `<!ENTITY ` + cur + ` "` + strings.Repeat("&"+prev+";", 10) + `">`
		prev = cur
	}
	laughs += `]><r>&` + prev + `;</r>`
	deep := strings.Repeat("<e>", 100000) + strings.Repeat("</e>", 100000)
	nested := `<xs:schema ` + xs + `><xs:element name="e"><xs:complexType><xs:sequence><xs:element ref="e" minOccurs="0"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	pattern := func(n int) string { return "<r>" + strings.Repeat("a", n) + "</r>" }
	patSchema := `<xs:schema ` + xs + `><xs:element name="r"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="(a|aa)*b"/></xs:restriction></xs:simpleType></xs:element></xs:schema>`

	probes := []probe{
		{name: "network: import from http://127.0.0.1 (requests seen)", schema: `<xs:schema ` + xs + ` xmlns:n="urn:n"><xs:import namespace="urn:n" schemaLocation="` + url + `"/><xs:element name="r" type="n:t"/></xs:schema>`, doc: `<r>ok</r>`},
		{name: "external entity (file:)", schema: str, doc: `<?xml version="1.0"?><!DOCTYPE r [<!ENTITY s SYSTEM "file://` + secret + `">]><r>&s;</r>`},
		{name: "billion laughs (10^9 bytes)", schema: str, doc: laughs},
		{name: "depth 100000", schema: nested, doc: deep},
		{name: "pattern (a|aa)*b, 20000 chars", schema: patSchema, doc: pattern(20000)},
		{name: "pattern (a|aa)*b, 40000 chars", schema: patSchema, doc: pattern(40000)},
		{name: "XSD 1.1 xs:assert in a schema", schema: `<xs:schema ` + xs + `><xs:element name="r"><xs:complexType><xs:sequence/><xs:assert test="true()"/></xs:complexType></xs:element></xs:schema>`, doc: `<r/>`},
		{name: "xs:redefine (doc invalid: 3 > maxInclusive 2)", schema: `<xs:schema ` + xs + `><xs:redefine schemaLocation="b.xsd"><xs:simpleType name="t"><xs:restriction base="t"><xs:maxInclusive value="2"/></xs:restriction></xs:simpleType></xs:redefine><xs:element name="r" type="t"/></xs:schema>`, doc: `<r>3</r>`,
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "b.xsd"), []byte(`<xs:schema `+xs+`><xs:simpleType name="t"><xs:restriction base="xs:int"/></xs:simpleType></xs:schema>`), 0o644)
			}},
		{name: "keyref to a missing key (doc invalid)", schema: `<xs:schema ` + xs + `><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="k" type="xs:string" maxOccurs="unbounded"/><xs:element name="ref" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType><xs:key name="K"><xs:selector xpath="k"/><xs:field xpath="."/></xs:key><xs:keyref name="R" refer="K"><xs:selector xpath="ref"/><xs:field xpath="."/></xs:keyref></xs:element></xs:schema>`, doc: `<r><k>a</k><ref>b</ref></r>`},
		{name: "xsi:type of a derived type (doc valid)", schema: `<xs:schema ` + xs + `><xs:complexType name="b"><xs:sequence><xs:element name="x" type="xs:string"/></xs:sequence></xs:complexType><xs:complexType name="d"><xs:complexContent><xs:extension base="b"><xs:sequence><xs:element name="y" type="xs:int"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType><xs:element name="r" type="b"/></xs:schema>`, doc: `<r xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="d"><x>s</x><y>1</y></r>`},
		{name: "substitution group member (doc valid)", schema: `<xs:schema ` + xs + `><xs:element name="h" type="xs:string"/><xs:element name="m" type="xs:string" substitutionGroup="h"/><xs:element name="r"><xs:complexType><xs:sequence><xs:element ref="h"/></xs:sequence></xs:complexType></xs:element></xs:schema>`, doc: `<r><m>x</m></r>`},
		{name: "error details: missing child, bad value", schema: `<xs:schema ` + xs + `><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a" type="xs:int"/><xs:element name="b" type="xs:string"/></xs:sequence></xs:complexType></xs:element></xs:schema>`, doc: "<r>\n  <a>x</a>\n</r>"},
	}
	var b strings.Builder
	for _, p := range probes {
		dir, err := os.MkdirTemp("", "goxsdgo-probe-")
		if err != nil {
			return err
		}
		os.WriteFile(filepath.Join(dir, "s.xsd"), []byte(p.schema), 0o644)
		os.WriteFile(filepath.Join(dir, "d.xml"), []byte(p.doc), 0o644)
		if p.setup != nil {
			p.setup(dir)
		}
		fmt.Fprintf(&b, "## %s\n", p.name)
		for _, n := range names {
			hits.Store(0)
			var res string
			start := time.Now()
			ok := withTimeout(func() { res = defaultRun(n, dir) })
			d := time.Since(start)
			if !ok {
				res = "timeout"
			}
			line := fmt.Sprintf("%-14s %8.1f ms  %s", n, float64(d.Microseconds())/1000, res)
			if strings.HasPrefix(p.name, "network") {
				line += fmt.Sprintf("  [http requests: %d]", hits.Load())
			}
			fmt.Fprintln(&b, line)
		}
		os.RemoveAll(dir)
	}
	b.WriteString("## error details (structured fields)\n")
	b.WriteString(errorDetails())
	fmt.Print(b.String())
	return writeOut(out, "probes.txt", b.String())
}

// errorDetails wypisuje, jakie pola strukturalne podaje błąd każdego kandydata (przez publiczne API).
func errorDetails() string {
	const xs = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`
	schema := `<xs:schema ` + xs + `><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a" type="xs:int"/><xs:element name="b" type="xs:string"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	doc := "<r>\n  <a>x</a>\n</r>"
	dir, _ := os.MkdirTemp("", "goxsdgo-probe-")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "s.xsd"), []byte(schema), 0o644)
	var b strings.Builder
	if set, err := xsd.Compile(os.DirFS(dir), "s.xsd"); err == nil {
		errs, _ := set.Validate(strings.NewReader(doc))
		for _, e := range errs {
			fmt.Fprintf(&b, "goxsdgo        code=%s line=%d col=%d path=%s expected=%v\n", e.Code, e.Line, e.Column, e.Path, e.Expected)
		}
	}
	if eng, err := jacoelho.Compile(jacoelho.Bytes("s.xsd", []byte(schema))); err == nil {
		for _, d := range xsderrors.Flatten(eng.Validate(strings.NewReader(doc))) {
			var xe *xsderrors.Error
			if errors.As(d, &xe) {
				fmt.Fprintf(&b, "jacoelho/xsd   code=%s line=%d col=%d path=%s msg=%s\n", xe.Code(), xe.Line(), xe.Column(), xe.Path(), oneLine(xe.Message()))
			}
		}
	}
	os.WriteFile(filepath.Join(dir, "d.xml"), []byte(doc), 0o644)
	if s, err := cbsigma.Load(os.DirFS(dir), "s.xsd"); err == nil {
		var list *cbsigma.Errors
		if errors.As(s.Validate([]byte(doc)), &list) {
			for _, i := range *list {
				fmt.Fprintf(&b, "cB-Sigma/xsd   line=%d path=%s kind=%d msg=%s\n", i.Line, i.Path, i.Kind, oneLine(i.Msg))
			}
		}
	}
	if s, err := knroy.LoadFile(filepath.Join(dir, "s.xsd"), knroy.Options{}); err == nil {
		tree, err := knroy.ParseDocument(strings.NewReader(doc), xdm.ParseOptions{TrackPositions: true})
		if err == nil {
			if verr := s.Validate(tree.Root, knroy.ValidateOptions{}); verr != nil {
				fmt.Fprintf(&b, "knroy/go-xml   %T: %s\n", verr, oneLine(verr.Error()))
			}
		}
	}
	cmd := exec.Command(xmllint, "--noout", "--schema", filepath.Join(dir, "s.xsd"), filepath.Join(dir, "d.xml"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Run()
	fmt.Fprintf(&b, "libxml2        %s\n", oneLine(stderr.String()))
	return b.String()
}
