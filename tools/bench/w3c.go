package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/internal/w3c"
)

// caseTimeout ogranicza czas jednego przypadku: kandydat, który go przekroczy, nie wydaje werdyktu.
const caseTimeout = 30 * time.Second

// w3cResult to wynik kandydata na zestawie W3C.
type w3cResult struct {
	Candidate     string            `json:"candidate"`
	InScope       int               `json:"in_scope"`
	Passed        int               `json:"passed"`
	SchemaPassed  int               `json:"schema_passed"`
	SchemaInScope int               `json:"schema_in_scope"`
	InstPassed    int               `json:"instance_passed"`
	InstInScope   int               `json:"instance_in_scope"`
	NotComparable int               `json:"not_comparable"`
	Undecided     int               `json:"undecided"`
	TimedOut      int               `json:"timed_out"`
	ByContributor map[string][2]int `json:"by_contributor"` // [passed, in scope]
	Seconds       float64           `json:"seconds"`
}

func w3cEnv(root string) (env, error) {
	src, err := testkit.LoadSources(root)
	if err != nil {
		return env{}, err
	}
	urls := map[string]string{}
	for u, p := range src.Locations("w3c-schemas") {
		urls[u] = filepath.Join(root, "testdata", "w3c", "schemas", filepath.FromSlash(p))
	}
	return realEnv(env{dir: filepath.Join(root, "testdata", "w3c", "xsdtests"), urls: urls}), nil
}

// runW3C uruchamia kandydata na wszystkich przypadkach W3C w zakresie XSD 1.0 2e (internal/w3c, ta sama lista
// i te same reguły zaliczenia co TestW3CSuite).
func runW3C(root, out string, c candidate) (*w3cResult, error) {
	e, err := w3cEnv(root)
	if err != nil {
		return nil, err
	}
	cases, err := w3c.Load(os.DirFS(e.dir), w3c.Tokens1_0)
	if err != nil {
		return nil, err
	}
	schemaExpected := map[string]w3c.Validity{}
	for _, cs := range cases {
		if cs.Kind == w3c.SchemaTest {
			schemaExpected[cs.Set+"/"+cs.Group] = cs.Expected
		}
	}
	res := &w3cResult{Candidate: c.name(), ByContributor: map[string][2]int{}}
	plain := map[string]entry{}
	hinted := map[string]entry{}
	perInstance := c.name() == "cB-Sigma/xsd" // wskazówki tego kandydata wczytuje się razem z dokumentem
	var lines []string
	start := time.Now()
	for _, cs := range cases {
		if !cs.InScope() {
			continue
		}
		res.InScope++
		bc := res.ByContributor[cs.Contributor]
		bc[1]++
		var got string
		ok := withTimeout(func() {
			got = verdictW3C(c, e, cs, plain, hinted, perInstance, schemaExpected)
		})
		if !ok {
			got = "timeout"
			res.TimedOut++
		}
		switch got {
		case "not comparable":
			res.NotComparable++
		case "undecided":
			res.Undecided++
		}
		pass := (got == "valid" || got == "invalid") && cs.Matches(w3c.Validity(got))
		if pass {
			res.Passed++
			bc[0]++
		}
		if cs.Kind == w3c.SchemaTest {
			res.SchemaInScope++
			if pass {
				res.SchemaPassed++
			}
		} else {
			res.InstInScope++
			if pass {
				res.InstPassed++
			}
		}
		res.ByContributor[cs.Contributor] = bc
		lines = append(lines, cs.ID+"\t"+string(cs.Expected)+"\t"+got)
	}
	res.Seconds = time.Since(start).Seconds()
	if err := writeOut(out, "w3c-"+slug(c.name())+".tsv", "id\texpected\tverdict\n"+strings.Join(lines, "\n")+"\n"); err != nil {
		return nil, err
	}
	return res, nil
}

// verdictW3C zwraca "valid", "invalid", "undecided" albo "not comparable".
func verdictW3C(c candidate, e env, cs *w3c.Case, plain, hinted map[string]entry, perInstance bool,
	schemaExpected map[string]w3c.Validity) string {
	key := strings.Join(cs.SchemaDocs, "\x00")
	var set compiled
	var err error
	if cs.Kind == w3c.SchemaTest {
		if v, ok := plain[key]; ok {
			set, err = v.set, v.err
		} else {
			set, err = c.compile(e, cs.SchemaDocs)
			plain[key] = entry{set, err}
		}
	} else {
		hkey := key
		if perInstance {
			hkey += "\x00" + cs.Instance
		}
		if v, ok := hinted[hkey]; ok {
			set, err = v.set, v.err
		} else {
			set, err = c.compileHints(e, cs.SchemaDocs, cs.Instance)
			if !perInstance {
				hinted[hkey] = entry{set, err}
			}
		}
	}
	if err != nil && os.Getenv("BENCH_DEBUG") != "" && debugCount < 3 {
		debugCount++
		fmt.Fprintf(os.Stderr, "debug %s: %.400v\n", cs.ID, err)
	}
	switch {
	case errors.Is(err, errNotComparable):
		return "not comparable"
	case errors.Is(err, errUndecided):
		return "undecided"
	case err != nil:
		if cs.Kind == w3c.SchemaTest || schemaExpected[cs.Set+"/"+cs.Group] == w3c.Invalid {
			return "invalid"
		}
		return "undecided" // odrzucony schemat poprawny nie daje werdyktu dla dokumentu (jak TestW3CSuite)
	case cs.Kind == w3c.SchemaTest:
		return "valid"
	}
	ok, err := set.validate(e.abs(cs.Instance))
	switch {
	case errors.Is(err, errUndecided):
		return "undecided"
	case err != nil:
		return "undecided"
	case ok:
		return "valid"
	}
	return "invalid"
}

var debugCount int

// entry to skompilowany schemat w pamięci podręcznej przebiegu.
type entry struct {
	set compiled
	err error
}

func withTimeout(f func()) bool {
	done := make(chan struct{})
	go func() {
		defer func() { recover(); close(done) }()
		f()
	}()
	select {
	case <-done:
		return true
	case <-time.After(caseTimeout):
		return false
	}
}

func slug(s string) string {
	return strings.NewReplacer("/", "-", " ", "-").Replace(strings.ToLower(s))
}

func writeOut(dir, name, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func printW3C(r *w3cResult) {
	fmt.Printf("%-14s passed %5d / %5d (schema %5d / %5d, instance %5d / %5d); not comparable %d, undecided %d, timeout %d; %.0f s\n",
		r.Candidate, r.Passed, r.InScope, r.SchemaPassed, r.SchemaInScope, r.InstPassed, r.InstInScope,
		r.NotComparable, r.Undecided, r.TimedOut, r.Seconds)
	names := make([]string, 0, len(r.ByContributor))
	for n := range r.ByContributor {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		v := r.ByContributor[n]
		fmt.Printf("    %-10s %5d / %5d\n", n, v[0], v[1])
	}
}
