package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
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

// perfSummary: mediana i rozrzut z kilku przebiegów jednego kandydata na jednym rozmiarze.
type perfSummary struct {
	Candidate     string       `json:"candidate"`
	MB            int          `json:"mb"`
	Runs          int          `json:"runs"`
	MedianSeconds float64      `json:"median_seconds"`
	MinSeconds    float64      `json:"min_seconds"`
	MaxSeconds    float64      `json:"max_seconds"`
	MedianRSSMB   float64      `json:"median_max_rss_mb"`
	Valid         string       `json:"verdict"`
	Each          []perfResult `json:"each"`
}

func median(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func summarize(rs []perfResult) perfSummary {
	s := perfSummary{Candidate: rs[0].Candidate, MB: rs[0].MB, Runs: len(rs), Valid: rs[0].Valid, Each: rs}
	var t, m []float64
	s.MinSeconds = rs[0].Seconds
	for _, r := range rs {
		t = append(t, r.Seconds)
		m = append(m, r.MaxRSSMB)
		s.MinSeconds = min(s.MinSeconds, r.Seconds)
		s.MaxSeconds = max(s.MaxSeconds, r.Seconds)
		if r.Valid != s.Valid {
			s.Valid = "inconsistent"
		}
	}
	s.MedianSeconds, s.MedianRSSMB = median(t), median(m)
	return s
}

// maxRSSMB: największy zestaw rezydentny procesu potomnego z rusage (macOS: bajty, Linux: KiB).
func maxRSSMB(ps *os.ProcessState) float64 {
	ru, ok := ps.SysUsage().(*syscall.Rusage)
	if !ok || ru == nil {
		return 0
	}
	b := float64(ru.Maxrss)
	if runtime.GOOS != "darwin" {
		b *= 1024
	}
	return b / (1 << 20)
}

// runPerf: każdy kandydat w osobnym procesie (`bench perf-one`), runs razy na każdy rozmiar, z medianą
// i rozrzutem; pamięć to największy zestaw rezydentny procesu (rusage). libxml2 jako xmllint --stream.
// Rozmiary w MB jako argumenty (domyślnie 20 i 200; 200 tylko dla walidatorów strumieniowych, bo pozostałe
// budują drzewo całego dokumentu). Kolejność przebiegów jest przeplatana (kandydaci na zmianę), żeby
// zmiana obciążenia maszyny rozkładała się na wszystkich.
func runPerf(root, out string, cands []candidate, args []string, runs int) error {
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
	if runs < 1 {
		runs = 1
	}
	var all []perfSummary
	for _, mb := range sizes {
		doc := filepath.Join(out, fmt.Sprintf("jpk_v7m_3_%dmb.xml", mb))
		if _, err := os.Stat(doc); err != nil {
			os.MkdirAll(out, 0o755)
			if err := syntheticJPK(root, mb, doc); err != nil {
				return err
			}
		}
		byCand := map[string][]perfResult{}
		var order []string
		for i := 0; i < runs; i++ {
			for _, c := range cands {
				streaming := c.name() == "goxsdgo" || c.name() == "jacoelho/xsd"
				if mb > 100 && !streaming {
					continue
				}
				r := perfResult{Candidate: c.name(), MB: mb}
				cmd := exec.Command(self, "-root", root, "perf-one", c.name(), doc)
				// jacoelho/xsd domyślnie zatrzymuje się po 64 MiB wejścia; w pomiarze limit jest podniesiony.
				cmd.Env = append(os.Environ(), "BENCH_RAISE_LIMITS=1")
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err := cmd.Run()
				fmt.Sscan(strings.TrimSpace(stdout.String()), &r.Seconds, &r.Valid)
				if cmd.ProcessState != nil {
					r.MaxRSSMB = maxRSSMB(cmd.ProcessState)
				}
				if err != nil {
					r.Note = strings.TrimSpace(lastLine(stderr.String()))
				}
				fmt.Printf("%-14s %4d MB run %d: %6.2f s, max RSS %7.1f MB, %s %s\n", r.Candidate, mb, i+1, r.Seconds, r.MaxRSSMB, r.Valid, r.Note)
				if byCand[r.Candidate] == nil {
					order = append(order, r.Candidate)
				}
				byCand[r.Candidate] = append(byCand[r.Candidate], r)
			}
			// libxml2: xmllint --stream z katalogiem XML (importy MF bez sieci)
			if r, err := perfXmllint(root, out, doc, mb); err == nil {
				fmt.Printf("%-14s %4d MB run %d: %6.2f s, max RSS %7.1f MB, %s %s\n", r.Candidate, mb, i+1, r.Seconds, r.MaxRSSMB, r.Valid, r.Note)
				if byCand[r.Candidate] == nil {
					order = append(order, r.Candidate)
				}
				byCand[r.Candidate] = append(byCand[r.Candidate], r)
			} else {
				fmt.Println("xmllint:", err)
			}
		}
		for _, name := range order {
			s := summarize(byCand[name])
			fmt.Printf("%-14s %4d MB: median %6.2f s (%.2f–%.2f, %d runs), median max RSS %7.1f MB, %s\n",
				s.Candidate, mb, s.MedianSeconds, s.MinSeconds, s.MaxSeconds, s.Runs, s.MedianRSSMB, s.Valid)
			all = append(all, s)
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

func perfXmllint(root, out, doc string, mb int) (perfResult, error) {
	r := perfResult{Candidate: "libxml2", MB: mb}
	catalog, err := writeCatalogFile(root, out)
	if err != nil {
		return r, err
	}
	e, _, _ := mfEnv(root)
	cmd := exec.Command(xmllint, "--stream", "--noout", "--nonet", "--schema", e.abs("jpk_v7m_3.xsd"), doc)
	cmd.Env = append(os.Environ(), "XML_CATALOG_FILES="+catalog)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	err = cmd.Run()
	r.Seconds = time.Since(start).Seconds()
	if cmd.ProcessState != nil {
		r.MaxRSSMB = maxRSSMB(cmd.ProcessState)
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
