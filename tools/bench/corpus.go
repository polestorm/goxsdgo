package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/polestorm/goxsdgo/internal/testkit"
)

type corpusResult struct {
	Candidate      string   `json:"candidate"`
	Schemas        int      `json:"schemas"`
	SchemasOK      int      `json:"schemas_ok"`
	Documents      int      `json:"documents"`
	DocumentsOK    int      `json:"documents_ok"`
	FirstFailure   string   `json:"first_failure,omitempty"`
	SchemaFailures int      `json:"schema_failures"`
	Failed         []string `json:"failed,omitempty"`
}

func mfEnv(root string) (env, *testkit.Sources, error) {
	src, err := testkit.LoadSources(root)
	if err != nil {
		return env{}, nil, err
	}
	dir := filepath.Join(root, "testdata", "schemas", "mf")
	urls := map[string]string{}
	for u, p := range src.Locations("mf-schemas") {
		urls[u] = filepath.Join(dir, filepath.FromSlash(p))
	}
	return realEnv(env{dir: dir, urls: urls}), src, nil
}

// runCorpus: każdy schemat MF z manifestu (bez brakujących plików local_only) i dokumenty
// z testdata/instances/expected.json z ich oczekiwanym werdyktem.
func runCorpus(root string, c candidate) (*corpusResult, error) {
	e, src, err := mfEnv(root)
	if err != nil {
		return nil, err
	}
	res := &corpusResult{Candidate: c.name()}
	var paths []string
	for _, ds := range src.Sets {
		if ds.Name != "mf-schemas" {
			continue
		}
		for _, f := range ds.Files {
			if _, err := os.Stat(e.abs(f.Path)); err != nil {
				continue
			}
			paths = append(paths, f.Path)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		res.Schemas++
		var err error
		ok := withTimeout(func() { _, err = c.compile(e, []string{p}) })
		if ok && err == nil {
			res.SchemasOK++
		} else {
			res.SchemaFailures++
			res.Failed = append(res.Failed, p)
			if res.FirstFailure == "" {
				if !ok {
					err = errors.New("timeout")
				}
				res.FirstFailure = fmt.Sprintf("%s: %.200v", p, err)
			}
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "testdata", "instances", "expected.json"))
	if err != nil {
		return nil, err
	}
	var expected []struct {
		Path   string `json:"path"`
		Schema string `json:"schema"`
		Valid  bool   `json:"valid"`
	}
	if err := json.Unmarshal(b, &expected); err != nil {
		return nil, err
	}
	for _, x := range expected {
		if x.Schema == "" {
			continue
		}
		res.Documents++
		set, err := c.compile(e, []string{x.Schema})
		if err != nil {
			continue
		}
		valid, err := set.validate(filepath.Join(root, "testdata", "instances", filepath.FromSlash(x.Path)))
		if err == nil && valid == x.Valid {
			res.DocumentsOK++
		}
	}
	return res, nil
}
