package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Syntetyczny JPK_V7M(3): przykład MF (testdata/instances/mf/jpk_v7m_3_organization.xml) z wierszem
// <SprzedazWiersz> powielonym do zadanego rozmiaru. Dane są fikcyjne (przykład MF).
func syntheticJPK(root string, mb int, path string) error {
	src, err := os.ReadFile(filepath.Join(root, "testdata", "instances", "mf", "jpk_v7m_3_organization.xml"))
	if err != nil {
		return err
	}
	open, close := []byte("<SprzedazWiersz>"), []byte("</SprzedazWiersz>")
	i := bytes.Index(src, open)
	j := bytes.LastIndex(src, close)
	if i < 0 || j < 0 {
		return fmt.Errorf("sample without SprzedazWiersz")
	}
	// wcięcie przed pierwszym wierszem należy do nagłówka
	for i > 0 && (src[i-1] == '\t' || src[i-1] == ' ') {
		i--
	}
	j += len(close)
	k := bytes.Index(src[i:], close) + i + len(close)
	head, row, tail := src[:i], src[i:k], src[j:]
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.Write(head)
	target := int64(mb) << 20
	n := int64(len(head) + len(tail))
	for n < target {
		f.Write(row)
		n += int64(len(row))
	}
	_, err = f.Write(tail)
	return err
}

type perfResult struct {
	Candidate string  `json:"candidate"`
	MB        int     `json:"mb"`
	Seconds   float64 `json:"seconds"`
	MaxRSSMB  float64 `json:"max_rss_mb"`
	Valid     string  `json:"verdict"`
	Note      string  `json:"note,omitempty"`
}

var rssRe = regexp.MustCompile(`(\d+)\s+maximum resident set size`)

// runPerf: każdy kandydat w osobnym procesie (`bench perf-one`) pod /usr/bin/time -l; libxml2 jako
// xmllint --stream. Rozmiary w MB jako argumenty (domyślnie 20 i 200; 200 tylko dla walidatorów
// strumieniowych, bo pozostałe budują drzewo całego dokumentu).
func runPerf(root, out string, cands []candidate, args []string) error {
	sizes := []int{20, 200}
	if len(args) > 0 {
		sizes = nil
		for _, a := range args {
			n, err := strconv.Atoi(a)
			if err != nil {
				return err
			}
			sizes = append(sizes, n)
		}
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	var all []perfResult
	for _, mb := range sizes {
		doc := filepath.Join(out, fmt.Sprintf("jpk_v7m_3_%dmb.xml", mb))
		if _, err := os.Stat(doc); err != nil {
			os.MkdirAll(out, 0o755)
			if err := syntheticJPK(root, mb, doc); err != nil {
				return err
			}
		}
		for _, c := range cands {
			streaming := c.name() == "goxsdgo" || c.name() == "jacoelho/xsd"
			if mb > 100 && !streaming {
				continue
			}
			r := perfResult{Candidate: c.name(), MB: mb}
			cmd := exec.Command("/usr/bin/time", "-l", self, "-root", root, "perf-one", c.name(), doc)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			fmt.Sscan(strings.TrimSpace(stdout.String()), &r.Seconds, &r.Valid)
			if m := rssRe.FindStringSubmatch(stderr.String()); m != nil {
				b, _ := strconv.ParseFloat(m[1], 64)
				r.MaxRSSMB = b / (1 << 20)
			}
			if err != nil {
				r.Note = strings.TrimSpace(lastLine(stderr.String()))
			}
			fmt.Printf("%-14s %4d MB: %6.2f s, max RSS %7.1f MB, %s %s\n", r.Candidate, mb, r.Seconds, r.MaxRSSMB, r.Valid, r.Note)
			all = append(all, r)
		}
		// libxml2: xmllint --stream z katalogiem XML (importy MF bez sieci)
		if r, err := perfXmllint(root, out, doc, mb); err == nil {
			fmt.Printf("%-14s %4d MB: %6.2f s, max RSS %7.1f MB, %s %s\n", r.Candidate, mb, r.Seconds, r.MaxRSSMB, r.Valid, r.Note)
			all = append(all, r)
		} else {
			fmt.Println("xmllint:", err)
		}
	}
	return writeJSON(out, "perf.json", all)
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for k := len(lines) - 1; k >= 0; k-- {
		if !strings.Contains(lines[k], "  ") && lines[k] != "" {
			return lines[k]
		}
	}
	return ""
}

// perfOne: kompilacja schematu JPK_V7M(3) i walidacja dokumentu w tym procesie; wypisuje czas całości i werdykt.
func perfOne(root string, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: perf-one CANDIDATE DOC")
	}
	var c candidate
	for _, x := range candidates() {
		if x.name() == args[0] {
			c = x
		}
	}
	if c == nil {
		return fmt.Errorf("unknown candidate %s", args[0])
	}
	e, _, err := mfEnv(root)
	if err != nil {
		return err
	}
	start := time.Now()
	set, err := c.compile(e, []string{"jpk_v7m_3.xsd"})
	if err != nil {
		return err
	}
	if js, ok := set.(jacoelhoSet); ok && os.Getenv("BENCH_RAISE_LIMITS") != "" {
		js.maxBytes = 1 << 40
		set = js
	}
	ok, err := set.validate(args[1])
	verdict := "valid"
	if err != nil {
		fmt.Fprintln(os.Stderr, "undecided:", oneLine(err.Error()))
		verdict = "undecided"
	} else if !ok {
		verdict = "invalid"
	}
	fmt.Printf("%.3f %s\n", time.Since(start).Seconds(), verdict)
	return nil
}

const xmllint = "/opt/homebrew/opt/libxml2/bin/xmllint"

func perfXmllint(root, out, doc string, mb int) (perfResult, error) {
	r := perfResult{Candidate: "libxml2", MB: mb}
	catalog, err := writeCatalogFile(root, out)
	if err != nil {
		return r, err
	}
	e, _, _ := mfEnv(root)
	cmd := exec.Command("/usr/bin/time", "-l", xmllint, "--stream", "--noout", "--nonet", "--schema", e.abs("jpk_v7m_3.xsd"), doc)
	cmd.Env = append(os.Environ(), "XML_CATALOG_FILES="+catalog)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	err = cmd.Run()
	r.Seconds = time.Since(start).Seconds()
	if m := rssRe.FindStringSubmatch(stderr.String()); m != nil {
		b, _ := strconv.ParseFloat(m[1], 64)
		r.MaxRSSMB = b / (1 << 20)
	}
	r.Valid = "valid"
	if err != nil {
		r.Valid = "invalid"
		r.Note = firstLine(stderr.String())
	}
	return r, nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

// writeCatalogFile zapisuje katalog XML (OASIS) dla xmllint: adresy schematów MF → pliki lokalne.
func writeCatalogFile(root, out string) (string, error) {
	e, _, err := mfEnv(root)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>` + "\n" + `<catalog xmlns="urn:oasis:names:tc:entity:xmlns:xml:catalog">` + "\n")
	for u, p := range e.urls {
		b.WriteString(fmt.Sprintf("  <system systemId=%q uri=%q/>\n  <uri name=%q uri=%q/>\n", u, "file://"+p, u, "file://"+p))
	}
	b.WriteString("</catalog>\n")
	path := filepath.Join(out, "mf-catalog.xml")
	return path, writeOut(out, "mf-catalog.xml", b.String())
}
