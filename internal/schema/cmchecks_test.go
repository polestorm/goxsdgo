package schema

import "testing"

// restriction buduje schemat z typem bazowym base i jego restrykcją derived (treść jako ciało complexType).
func restriction(base, derived string) map[string]string {
	return one(`<xs:complexType name="B">` + base + `</xs:complexType>
		<xs:complexType name="R"><xs:complexContent><xs:restriction base="B">` + derived + `</xs:restriction></xs:complexContent></xs:complexType>`)
}

const rcode = CodeRestriction + ".5.4.2"

// REQ: CM-007
func TestSchemaUPA(t *testing.T) {
	run(t, []schemaCase{
		{name: "optional then same", files: one(`<xs:complexType name="c"><xs:sequence><xs:element name="a" minOccurs="0"/><xs:element name="a"/></xs:sequence></xs:complexType>`), want: CodeNonambig},
		{name: "counted then same", files: one(`<xs:complexType name="c"><xs:sequence><xs:element name="a" minOccurs="2" maxOccurs="2"/><xs:element name="a"/></xs:sequence></xs:complexType>`)},
		{name: "any then element", files: one(`<xs:complexType name="c"><xs:sequence><xs:any minOccurs="0" processContents="lax"/><xs:element name="a"/></xs:sequence></xs:complexType>`), want: CodeNonambig},
		{name: "substitution overlap", files: one(`<xs:element name="h"/><xs:element name="m" substitutionGroup="h"/>
			<xs:complexType name="c"><xs:choice><xs:element ref="h"/><xs:element ref="m"/></xs:choice></xs:complexType>`), want: CodeNonambig},
		{name: "abstract head and member", files: one(`<xs:element name="h" abstract="true"/><xs:element name="m" substitutionGroup="h"/>
			<xs:complexType name="c"><xs:sequence><xs:element ref="h" minOccurs="0"/><xs:element name="x"/></xs:sequence></xs:complexType>`)},
		{name: "extension appends", files: one(`<xs:complexType name="b"><xs:sequence><xs:element name="a" minOccurs="0"/></xs:sequence></xs:complexType>
			<xs:complexType name="c"><xs:complexContent><xs:extension base="b"><xs:sequence><xs:element name="a"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>`), want: CodeNonambig},
	})
}

// REQ: CM-008
func TestSchemaEDC(t *testing.T) {
	run(t, []schemaCase{
		{name: "different types", files: one(`<xs:complexType name="c"><xs:choice><xs:element name="a" type="xs:int"/><xs:element name="a" type="xs:string"/></xs:choice></xs:complexType>`), want: CodeElementConsistent},
		{name: "same named type", files: one(`<xs:complexType name="c"><xs:sequence><xs:element name="a" type="xs:int"/><xs:element name="b"/><xs:element name="a" type="xs:int"/></xs:sequence></xs:complexType>`)},
		{name: "anonymous types", files: one(`<xs:complexType name="c"><xs:sequence><xs:element name="a"><xs:simpleType><xs:restriction base="xs:int"/></xs:simpleType></xs:element><xs:element name="b"/>
			<xs:element name="a"><xs:simpleType><xs:restriction base="xs:int"/></xs:simpleType></xs:element></xs:sequence></xs:complexType>`), want: CodeElementConsistent},
		{name: "same global twice", files: one(`<xs:element name="g"><xs:complexType/></xs:element>
			<xs:complexType name="c"><xs:sequence><xs:element ref="g"/><xs:element name="b"/><xs:element ref="g"/></xs:sequence></xs:complexType>`)},
		{name: "implicit via substitution group", files: one(`<xs:element name="h" type="xs:string"/><xs:element name="m" substitutionGroup="h" type="xs:token"/>
			<xs:complexType name="c"><xs:sequence><xs:element ref="h"/><xs:element name="m" type="xs:int"/></xs:sequence></xs:complexType>`), want: CodeElementConsistent},
	})
}

// REQ: CM-009, CM-010, CM-011, CM-012
func TestSchemaParticleRestriction(t *testing.T) {
	s := func(body string) string { return `<xs:sequence>` + body + `</xs:sequence>` }
	c := func(body string, attrs string) string { return `<xs:choice` + attrs + `>` + body + `</xs:choice>` }
	run(t, []schemaCase{
		// NameAndTypeOK
		{name: "narrower occurrence", files: restriction(s(`<xs:element name="a" maxOccurs="5"/>`), s(`<xs:element name="a" minOccurs="2" maxOccurs="3"/>`))},
		{name: "wider occurrence", files: restriction(s(`<xs:element name="a" maxOccurs="5"/>`), s(`<xs:element name="a" maxOccurs="6"/>`)), want: rcode},
		{name: "other name", files: restriction(s(`<xs:element name="a"/>`), s(`<xs:element name="b"/>`)), want: rcode},
		{name: "type restricted", files: restriction(s(`<xs:element name="a" type="xs:decimal"/>`), s(`<xs:element name="a" type="xs:int"/>`))},
		{name: "type widened", files: restriction(s(`<xs:element name="a" type="xs:int"/>`), s(`<xs:element name="a" type="xs:decimal"/>`)), want: rcode},
		{name: "nillable added", files: restriction(s(`<xs:element name="a"/>`), s(`<xs:element name="a" nillable="true"/>`)), want: rcode},
		{name: "fixed kept", files: restriction(s(`<xs:element name="a" type="xs:int" fixed="1"/>`), s(`<xs:element name="a" type="xs:int" fixed="01"/>`))},
		{name: "fixed dropped", files: restriction(s(`<xs:element name="a" type="xs:int" fixed="1"/>`), s(`<xs:element name="a" type="xs:int"/>`)), want: rcode},
		// NSCompat, NSSubset
		{name: "element for wildcard", files: restriction(s(`<xs:any maxOccurs="2" processContents="lax"/>`), s(`<xs:element name="a" maxOccurs="2"/>`))},
		{name: "element outside wildcard", files: one(`<xs:complexType name="B"><xs:sequence><xs:any namespace="##other"/></xs:sequence></xs:complexType>
			<xs:complexType name="R"><xs:complexContent><xs:restriction base="B"><xs:sequence><xs:element name="a"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`), want: rcode},
		{name: "wildcard subset", files: restriction(s(`<xs:any namespace="##any" processContents="lax"/>`), s(`<xs:any namespace="##local" processContents="strict"/>`))},
		{name: "wildcard weaker", files: restriction(s(`<xs:any processContents="strict"/>`), s(`<xs:any processContents="skip"/>`)), want: rcode},
		{name: "wildcard for element", files: restriction(s(`<xs:element name="a"/>`), s(`<xs:any/>`)), want: rcode},
		// NSRecurseCheckCardinality
		{name: "group for wildcard", files: restriction(s(`<xs:any minOccurs="3" maxOccurs="3" processContents="lax"/>`), `<xs:all><xs:element name="a"/><xs:element name="b"/><xs:element name="c"/></xs:all>`)},
		{name: "group too large for wildcard", files: restriction(s(`<xs:any minOccurs="1" maxOccurs="2" processContents="lax"/>`), s(`<xs:element name="a"/><xs:element name="b"/><xs:element name="c"/>`)), want: rcode},
		// Recurse
		{name: "drop optional", files: restriction(s(`<xs:element name="a"/><xs:element name="b" minOccurs="0"/><xs:element name="c"/>`), s(`<xs:element name="a"/><xs:element name="c"/>`))},
		{name: "drop required", files: restriction(s(`<xs:element name="a"/><xs:element name="b"/>`), s(`<xs:element name="a"/>`)), want: rcode},
		{name: "reorder", files: restriction(s(`<xs:element name="a"/><xs:element name="b"/>`), s(`<xs:element name="b"/><xs:element name="a"/>`)), want: rcode},
		// RecurseLax
		{name: "choice subset", files: restriction(c(`<xs:element name="a"/><xs:element name="b"/><xs:element name="c"/>`, ``), c(`<xs:element name="a"/><xs:element name="c"/>`, ``))},
		{name: "choice reordered", files: restriction(c(`<xs:element name="a"/><xs:element name="b"/>`, ``), c(`<xs:element name="b"/><xs:element name="a"/>`, ``)), want: rcode},
		// RecurseAsIfGroup
		{name: "element for choice", files: restriction(c(`<xs:element name="a"/><xs:element name="b"/>`, ``), s(`<xs:element name="b"/>`))},
		{name: "optional element for optional choice", files: restriction(s(c(`<xs:element name="a"/><xs:element name="b"/>`, ` minOccurs="0"`)), s(`<xs:element name="a" minOccurs="0"/>`)), want: rcode},
		// RecurseUnordered
		{name: "sequence for all", files: restriction(`<xs:all><xs:element name="a"/><xs:element name="b" minOccurs="0"/><xs:element name="c"/></xs:all>`, s(`<xs:element name="c"/><xs:element name="a"/>`))},
		{name: "sequence for all missing required", files: restriction(`<xs:all><xs:element name="a"/><xs:element name="c"/></xs:all>`, s(`<xs:element name="a"/>`)), want: rcode},
		// MapAndSum
		{name: "sequence for repeated choice", files: restriction(c(`<xs:element name="a"/><xs:element name="b"/>`, ` maxOccurs="2"`), s(`<xs:element name="a"/><xs:element name="b"/>`))},
		{name: "sequence too long for choice", files: restriction(c(`<xs:element name="a"/><xs:element name="b"/>`, ``), s(`<xs:element name="a"/><xs:element name="b"/>`)), want: rcode},
		// Forbidden
		{name: "choice for sequence", files: restriction(s(`<xs:element name="a"/><xs:element name="b"/>`), c(`<xs:element name="a"/><xs:element name="b"/>`, ``)), want: rcode},
		// Punkt 2.1: głowa grupy podstawienia jak choice.
		{name: "member for head", files: one(`<xs:element name="h"/><xs:element name="m" substitutionGroup="h"/>
			<xs:complexType name="B"><xs:sequence><xs:element ref="h"/></xs:sequence></xs:complexType>
			<xs:complexType name="R"><xs:complexContent><xs:restriction base="B"><xs:sequence><xs:element ref="m"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`)},
		// src-redefine.6.2.2
		{name: "redefine group narrower", files: map[string]string{
			"main.xsd": doc(`<xs:redefine schemaLocation="b.xsd"><xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group></xs:redefine>`),
			"b.xsd":    doc(`<xs:group name="g"><xs:sequence><xs:element name="a"/><xs:element name="b" minOccurs="0"/></xs:sequence></xs:group>`)}},
		{name: "redefine group wider", files: map[string]string{
			"main.xsd": doc(`<xs:redefine schemaLocation="b.xsd"><xs:group name="g"><xs:sequence><xs:element name="a"/><xs:element name="c"/></xs:sequence></xs:group></xs:redefine>`),
			"b.xsd":    doc(`<xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`)}, want: CodeRedefine + ".6.2.2"},
	})
}

// REQ: CM-015
func TestEffectiveRange(t *testing.T) {
	x := newRestrictor(nil)
	cases := []struct {
		p        *Particle
		min, max int64
	}{
		{seq(2, 3, el("a", 1, 2), el("b", 0, 1)), 2, 9},
		{cho(1, 2, el("a", 1, 2), el("b", 3, 4)), 1, 8},
		{seq(1, Unbounded, el("a", 0, 1)), 0, Unbounded},
		{seq(1, Unbounded), 0, 0},
		{seq(1, 1<<62, el("a", 4, 4)), 4, 1<<63 - 1},
	}
	for i, c := range cases {
		var mn, mx int64
		if r := x.normalize(c.p, nil); r != nil {
			mn, mx = effectiveRange(r)
		}
		if mn != c.min || mx != c.max {
			t.Errorf("case %d: (%d, %d), want (%d, %d)", i, mn, mx, c.min, c.max)
		}
	}
}

// TestRecurseIsNotGreedy: odwzorowanie zachowujące kolejność szukamy wyczerpująco. R = (a), B = (a?, a, b?):
// zachłanne przypisanie R.a do pierwszego a zostawiłoby niepuste drugie a. REQ: CM-011
func TestRecurseIsNotGreedy(t *testing.T) {
	a := &ElementDecl{Name: QName{Local: "a"}}
	b := &ElementDecl{Name: QName{Local: "b"}}
	part := func(d *ElementDecl, mn, mx int64) *Particle { return &Particle{MinOccurs: mn, MaxOccurs: mx, Term: d} }
	r := seq(1, 1, part(a, 1, 1), part(b, 0, 1))
	base := seq(1, 1, part(a, 0, 1), part(a, 1, 1), part(b, 0, 1))
	if x := newRestrictor(nil); !x.validRestriction(r, base) {
		t.Fatalf("(a, b?) is a valid restriction of (a?, a, b?): %s", x.why)
	}
}
