package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/polestorm/goxsdgo/xsd"
)

// pairCounter liczy pary (kod goxsdgo, kod libxml2) na tych samych danych: podstawa empiryczna tabeli compat
// (ADR 0009, kryterium K8). Para „first” to pierwsze błędy obu walidatorów w przypadku odrzuconym przez oba;
// para „line” to pierwsze błędy obu na tym samym wierszu dokumentu (każdy wiersz przypadku liczony raz).
type pairCounter struct {
	n map[pairKey]*pairCount
}

type pairKey struct {
	kind, goCode string
	lib          int
}

type pairCount struct{ first, line int }

func newPairCounter() *pairCounter { return &pairCounter{n: map[pairKey]*pairCount{}} }

func (p *pairCounter) add(kind, goCode string, lib int, first bool) {
	if goCode == "" || lib == 0 {
		return
	}
	k := pairKey{kind, goCode, lib}
	c := p.n[k]
	if c == nil {
		c = &pairCount{}
		p.n[k] = c
	}
	if first {
		c.first++
	} else {
		c.line++
	}
}

// addDocument dopisuje pary dla dokumentu odrzuconego przez oba walidatory.
func (p *pairCounter) addDocument(gerrs []xsd.Error, lerrs []libError) {
	var lib []libError
	for _, e := range lerrs {
		if e.Severity >= sevError {
			lib = append(lib, e)
		}
	}
	if len(gerrs) == 0 || len(lib) == 0 {
		return
	}
	p.add(xsd.KindDocument, gerrs[0].Code, lib[0].Code, true)
	libByLine := map[int]int{}
	for _, e := range lib {
		if _, ok := libByLine[e.Line]; !ok && e.Line > 0 {
			libByLine[e.Line] = e.Code
		}
	}
	seen := map[int]bool{}
	for _, e := range gerrs {
		if seen[e.Line] {
			continue
		}
		seen[e.Line] = true
		if lc, ok := libByLine[e.Line]; ok {
			p.add(xsd.KindDocument, e.Code, lc, false)
		}
	}
}

func (p *pairCounter) write(path string) error {
	keys := make([]pairKey, 0, len(p.n))
	for k := range p.n {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.goCode != b.goCode {
			return a.goCode < b.goCode
		}
		return a.lib < b.lib
	})
	var lines []string
	for _, k := range keys {
		c := p.n[k]
		lines = append(lines, fmt.Sprintf("%s\t%s\t%d\t%d\t%d", k.kind, k.goCode, k.lib, c.first, c.line))
	}
	return writeLines(path, "kind\tgoxsdgo\tlibxml2\tfirst\tline", lines)
}

func readPairs(path string, into *pairCounter) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // nagłówek
	for sc.Scan() {
		p := strings.Split(sc.Text(), "\t")
		if len(p) != 5 {
			continue
		}
		lib, _ := strconv.Atoi(p[2])
		first, _ := strconv.Atoi(p[3])
		line, _ := strconv.Atoi(p[4])
		k := pairKey{p[0], p[1], lib}
		c := into.n[k]
		if c == nil {
			c = &pairCount{}
			into.n[k] = c
		}
		c.first += first
		c.line += line
	}
	return sc.Err()
}

// runCompat łączy pary z przebiegów w3c i corpus, wybiera dla każdego kodu goxsdgo najczęstszy kod libxml2
// (najpierw pary z tego samego wiersza, potem pierwsze błędy) i porównuje wynik z tabelą xsd.LibXML2Code.
// Zapisuje out/compare/compat.tsv z liczebnościami i alternatywami.
func runCompat(root, out string) error {
	pc := newPairCounter()
	for _, f := range []string{"code-pairs-w3c.tsv", "code-pairs-corpus.tsv", "code-pairs-probe.tsv"} {
		if err := readPairs(filepath.Join(out, f), pc); err != nil {
			return fmt.Errorf("%w (run the w3c, corpus and probe commands first)", err)
		}
	}
	known, err := readConsumerCodes(filepath.Join(root, "testdata", "libxml2-consumer-codes.tsv"))
	if err != nil {
		return err
	}
	type cand struct {
		lib         int
		first, line int
	}
	byCode := map[string][]cand{}
	kindOf := map[string]string{}
	for k, c := range pc.n {
		byCode[k.goCode] = append(byCode[k.goCode], cand{k.lib, c.first, c.line})
		kindOf[k.goCode] = k.kind
	}
	codes := make([]string, 0, len(byCode))
	for c := range byCode {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	var lines []string
	agree, differ, missing := 0, 0, 0
	for _, code := range codes {
		cs := byCode[code]
		sort.Slice(cs, func(i, j int) bool {
			if cs[i].line+cs[i].first != cs[j].line+cs[j].first {
				return cs[i].line+cs[i].first > cs[j].line+cs[j].first
			}
			return cs[i].lib < cs[j].lib
		})
		total := 0
		var alts []string
		for _, c := range cs {
			total += c.first + c.line
			alts = append(alts, fmt.Sprintf("%d:%d", c.lib, c.first+c.line))
		}
		best := cs[0]
		table, ok := xsd.LibXML2Code(code)
		status := "agree"
		switch {
		case !ok:
			status = "missing in table"
			missing++
		case table != best.lib:
			status = fmt.Sprintf("table has %d", table)
			differ++
		default:
			agree++
		}
		inMsg := "no"
		if known[best.lib] {
			inMsg = "yes"
		}
		lines = append(lines, fmt.Sprintf("%s\t%s\t%d\t%d/%d\t%s\t%s\t%s", kindOf[code], code, best.lib,
			best.first+best.line, total, inMsg, status, strings.Join(alts, " ")))
	}
	if err := writeLines(filepath.Join(out, "compat.tsv"),
		"kind\tgoxsdgo\tlibxml2 (most frequent)\tshare\tconsumer code\ttable\talternatives code:count", lines); err != nil {
		return err
	}
	fmt.Printf("goxsdgo codes observed with a libxml2 counterpart: %d; table agrees %d, differs %d, missing %d\n",
		len(codes), agree, differ, missing)
	return nil
}

// readConsumerCodes czyta kody libxml2, dla których odbiorca ma tłumaczenia (testdata/libxml2-consumer-codes.tsv:
// kod i opis rozdzielone tabulatorem, wiersze z # to komentarze).
func readConsumerCodes(path string) (map[int]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[int]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		code, _, _ := strings.Cut(line, "\t")
		if n, err := strconv.Atoi(strings.TrimSpace(code)); err == nil {
			out[n] = true
		}
	}
	return out, nil
}
