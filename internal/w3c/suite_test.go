package w3c

import (
	"os"
	"testing"
	"testing/fstest"
)

const ts = `xmlns="http://www.w3.org/XML/2004/xml-schema-test-suite/" xmlns:xlink="http://www.w3.org/1999/xlink"`

func TestLoadRules(t *testing.T) {
	fsys := fstest.MapFS{
		"suite.xml": {Data: []byte(`<testSuite ` + ts + `>
			<testSetRef xlink:href="msMeta/a.testSet"/>
			<testSetRef xlink:href="saxonMeta/only11.testSet"/></testSuite>`)},
		"msMeta/a.testSet": {Data: []byte(`<testSet ` + ts + ` name="A">
			<testGroup name="g1">
				<schemaTest name="s"><schemaDocument xlink:href="../msData/g1.xsd"/>
					<expected validity="valid"/><current status="accepted"/></schemaTest>
				<instanceTest name="i1"><instanceDocument xlink:href="../msData/g1.xml"/>
					<expected validity="invalid"/></instanceTest>
				<instanceTest name="i2" version="1.1"><instanceDocument xlink:href="../msData/g1b.xml"/>
					<expected validity="valid"/></instanceTest>
				<instanceTest name="i3"><instanceDocument xlink:href="../msData/g1c.xml"/>
					<expected validity="valid" version="1.1"/><expected validity="invalid" version="1.0"/></instanceTest>
				<instanceTest name="i4"><instanceDocument xlink:href="../msData/g1d.xml"/>
					<expected validity="valid" version="Unicode_4.0.0"/><expected validity="invalid" version="Unicode_6.0.0"/></instanceTest>
				<instanceTest name="i5"><instanceDocument xlink:href="../msData/g1e.xml"/>
					<expected validity="indeterminate"/></instanceTest>
			</testGroup>
			<testGroup name="g2" version="1.1">
				<schemaTest name="s"><schemaDocument xlink:href="../msData/g2.xsd"/><expected validity="valid"/></schemaTest>
			</testGroup>
			<testGroup name="g3" version="1.0 1.1">
				<schemaTest name="s"><schemaDocument xlink:href="../msData/g3.xsd"/><expected validity="invalid"/></schemaTest>
			</testGroup></testSet>`)},
		"msData/g1.xsd": {}, "msData/g1.xml": {}, "msData/g1b.xml": {}, "msData/g1c.xml": {}, "msData/g1d.xml": {},
		"msData/g1e.xml": {}, "msData/g2.xsd": {}, "msData/g3.xsd": {}, "saxonData/x.xsd": {},
		"saxonMeta/only11.testSet": {Data: []byte(`<testSet ` + ts + ` name="B" version="1.1">
			<testGroup name="g"><schemaTest name="s"><schemaDocument xlink:href="../saxonData/x.xsd"/>
				<expected validity="valid"/></schemaTest></testGroup></testSet>`)},
	}
	cases, err := Load(fsys, Tokens1_0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		expected Validity
		excluded string
	}{
		"A/g1/s":  {Valid, ""},
		"A/g1/i1": {Invalid, ""},
		"A/g1/i2": {"", "version 1.1"},
		"A/g1/i3": {Invalid, ""},
		"A/g1/i4": {"", "no expected outcome for XSD 1.0 2e"},
		"A/g1/i5": {"", "expected outcome indeterminate"},
		"A/g2/s":  {"", "version 1.1"},
		"A/g3/s":  {Invalid, ""},
		"B/g/s":   {"", "version 1.1"},
	}
	if len(cases) != len(want) {
		t.Fatalf("%d cases, want %d", len(cases), len(want))
	}
	for _, c := range cases {
		w, ok := want[c.ID]
		if !ok {
			t.Errorf("unexpected case %s", c.ID)
			continue
		}
		if c.Expected != w.expected || c.Excluded != w.excluded {
			t.Errorf("%s: expected %q excluded %q, want %q %q", c.ID, c.Expected, c.Excluded, w.expected, w.excluded)
		}
	}
	c := cases[1]
	if c.Contributor != "Microsoft" || c.Instance != "msData/g1.xml" || len(c.SchemaDocs) != 1 || c.SchemaDocs[0] != "msData/g1.xsd" {
		t.Errorf("A/g1/i1: %+v", c)
	}
	if cases[0].Status != "accepted" {
		t.Errorf("A/g1/s status %q", cases[0].Status)
	}
}

func TestLoadRejectsEscapingReference(t *testing.T) {
	fsys := fstest.MapFS{
		"suite.xml": {Data: []byte(`<testSuite ` + ts + `><testSetRef xlink:href="m/a.testSet"/></testSuite>`)},
		"m/a.testSet": {Data: []byte(`<testSet ` + ts + ` name="A"><testGroup name="g"><schemaTest name="s">
			<schemaDocument xlink:href="../../etc/passwd"/><expected validity="valid"/></schemaTest></testGroup></testSet>`)},
	}
	if _, err := Load(fsys, Tokens1_0); err == nil {
		t.Fatal("reference outside the suite accepted")
	}
}

// TestLoadSuite czyta prawdziwy zestaw, jeśli jest pobrany: identyfikatory są jednoznaczne (Load to sprawdza),
// każdy wskazany plik istnieje, a liczby zgadzają się z rozpoznaniem z 2026-11-25.
func TestLoadSuite(t *testing.T) {
	const root = "../../testdata/w3c/xsdtests"
	if _, err := os.Stat(root + "/suite.xml"); err != nil {
		t.Skip("W3C suite not fetched; run: go -C tools/fetch run . -root ../..")
	}
	fsys := os.DirFS(root)
	cases, err := Load(fsys, Tokens1_0)
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]bool{}
	for _, c := range cases {
		groups[c.Set+"/"+c.Group] = true
	}
	for g := range ExcludedGroups {
		if !groups[g] {
			t.Errorf("ExcludedGroups: no group %s in the suite", g)
		}
	}
	inScope := map[Kind]int{}
	excluded := map[string]int{}
	for _, c := range cases {
		if c.InScope() {
			inScope[c.Kind]++
		} else {
			excluded[c.Excluded]++
		}
	}
	for reason, n := range excluded {
		t.Logf("excluded %5d: %s", n, reason)
	}
	t.Logf("%d cases, in scope: %d schema tests, %d instance tests", len(cases), inScope[SchemaTest], inScope[InstanceTest])
	if inScope[SchemaTest] < 14000 || inScope[InstanceTest] < 24000 {
		t.Errorf("fewer cases in scope than expected: %v", inScope)
	}
}
