package xsd

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// goxmlgoModule to ścieżka modułu czytnika XML (ADR 0014).
const goxmlgoModule = "github.com/polestorm/goxmlgo"

var goxmlgoOnce struct {
	sync.Once
	dir, version string
	err          error
}

// goxmlgo zwraca katalog modułu goxmlgo w pamięci modułów i jego wersję z go.mod goxsdgo. Z nich testy księgowe
// czytają wymagania (requirements/) i kody błędów (xmlreader) goxmlgo, zamiast powtarzać je w goxsdgo.
func goxmlgo(t *testing.T) (dir, version string) {
	t.Helper()
	g := &goxmlgoOnce
	g.Do(func() {
		b, err := os.ReadFile("../go.mod")
		if err != nil {
			g.err = err
			return
		}
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "require "))
			if len(f) >= 2 && f[0] == goxmlgoModule {
				g.version = f[1]
			}
		}
		out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", goxmlgoModule).Output()
		if err != nil {
			g.err = err
			return
		}
		g.dir = strings.TrimSpace(string(out))
	})
	if g.err != nil || g.dir == "" || g.version == "" {
		t.Fatalf("module %s: dir %q, version %q in go.mod: %v", goxmlgoModule, g.dir, g.version, g.err)
	}
	return g.dir, g.version
}
