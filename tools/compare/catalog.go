package main

import (
	"encoding/json"
	"encoding/xml"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// writeCatalog zapisuje katalog XML (OASIS XML Catalogs) przypisujący adresom ze schemaLocation pliki z dysku.
// Zwraca ścieżkę pliku tymczasowego.
func writeCatalog(locations map[string]string, dir string) (string, error) {
	urls := make([]string, 0, len(locations))
	for u := range locations {
		urls = append(urls, u)
	}
	sort.Strings(urls)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>` + "\n" + `<catalog xmlns="urn:oasis:names:tc:entity:xmlns:xml:catalog">` + "\n")
	for _, u := range urls {
		file := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(dir, filepath.FromSlash(locations[u])))}).String()
		b.WriteString(`  <system systemId="` + attr(u) + `" uri="` + attr(file) + `"/>` + "\n")
		b.WriteString(`  <uri name="` + attr(u) + `" uri="` + attr(file) + `"/>` + "\n")
	}
	b.WriteString("</catalog>\n")
	f, err := os.CreateTemp("", "goxsdgo-catalog-*.xml")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(b.String()); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func attr(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return strings.ReplaceAll(b.String(), `"`, "&#34;")
}

// expectedInstance odpowiada wpisowi w testdata/instances/expected.json (xsd/corpus_test.go).
type expectedInstance struct {
	Path   string `json:"path"`
	Schema string `json:"schema"`
	Valid  bool   `json:"valid"`
}

func readExpected(path string) ([]expectedInstance, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var e []expectedInstance
	return e, json.Unmarshal(b, &e)
}
