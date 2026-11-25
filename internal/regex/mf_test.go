package regex

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/polestorm/goxsdgo/internal/testkit"
)

// TestMFPatterns kompiluje każdy wzorzec (xs:pattern) ze schematów MF z korpusu: schematy odbiorców muszą się
// kompilować (kryterium K2), a wzorce to ich najczęstsza konstrukcja z części 2.
//
// REQ: RE-008
func TestMFPatterns(t *testing.T) {
	const dir = "../../testdata/schemas/mf"
	if _, err := os.Stat(dir); err != nil {
		t.Skip(testkit.FetchHint)
	}
	seen := map[string]bool{}
	fsys := os.DirFS(dir)
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".xsd") {
			return err
		}
		f, err := fsys.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		dec := xml.NewDecoder(f)
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "pattern" && se.Name.Space == "http://www.w3.org/2001/XMLSchema" {
				for _, a := range se.Attr {
					if a.Name.Local == "value" && !seen[a.Value] {
						seen[a.Value] = true
						if _, err := Compile(a.Value); err != nil {
							t.Errorf("%s: %v", path.Base(p), err)
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d distinct MF patterns compiled", len(seen))
}
