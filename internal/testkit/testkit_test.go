package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recorder zbiera błędy zapadki zamiast kończyć test.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Errorf(format string, args ...any) { r.errors = append(r.errors, format) }
func (r *recorder) Logf(string, ...any)               {}
func (r *recorder) Helper()                           {}

func set(ids ...string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func TestRatchetPicksShorterListAndDetectsRegressions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.ratchet")
	scope := set("a", "b", "c", "d")

	// Mało zaliczonych: plik w trybie passing.
	Ratchet(t, path, "h", scope, set("a"), true)
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "# mode: passing\na\n") {
		t.Fatalf("passing mode expected:\n%s", b)
	}
	// Dużo zaliczonych: plik w trybie failing.
	Ratchet(t, path, "h", scope, set("a", "b", "c"), true)
	b, _ = os.ReadFile(path)
	if !strings.Contains(string(b), "# mode: failing\nd\n") {
		t.Fatalf("failing mode expected:\n%s", b)
	}
	// Regresja w trybie failing: "b" przechodził, teraz nie.
	r := &recorder{TB: t}
	Ratchet(r, path, "h", scope, set("a", "c"), false)
	if len(r.errors) != 1 {
		t.Fatalf("regression not reported: %v", r.errors)
	}
	// Nowe zaliczenie nie jest błędem.
	r = &recorder{TB: t}
	Ratchet(r, path, "h", scope, set("a", "b", "c", "d"), false)
	if len(r.errors) != 0 {
		t.Fatalf("improvement reported as error: %v", r.errors)
	}
}
