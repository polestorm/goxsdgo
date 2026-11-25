//go:build ignore

// gentables generuje tables.go: klasy znaków nazw XML (dla \i i \c) i bloki Unicode (dla \p{IsX}).
//
// Źródła (pobierane ręcznie, nie należą do repozytorium):
//   - XML 1.0 Second Edition, https://www.w3.org/TR/2000/REC-xml-20001006, dodatek B (produkcje [84]–[89]).
//     XSD 1.0 2e (część 2, F.1.1) definiuje \i i \c przez produkcje Letter i NameChar z XML 1.0 2e.
//   - Unicode 3.1 Blocks.txt, https://www.unicode.org/Public/3.1-Update/Blocks-4.txt. Tablica bloków
//     w dodatku F części 2 to bloki BMP z tego pliku bez bloków surogatów; generator to sprawdza, jeśli
//     podano też tekst części 2 (-xsd2).
//
// Użycie:
//
//	go run gentables.go -xml xml2e.html -blocks Blocks-4.txt -xsd2 REC-xmlschema-2-20041028.html
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"go/format"
	"html"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type rng struct{ lo, hi rune }

func main() {
	xmlPath := flag.String("xml", "", "XML 1.0 Second Edition (HTML)")
	blocksPath := flag.String("blocks", "", "Unicode 3.1 Blocks.txt")
	xsd2Path := flag.String("xsd2", "", "XML Schema Part 2 Second Edition (HTML), optional cross-check")
	out := flag.String("o", "tables.go", "output file")
	flag.Parse()

	xmlSrc := read(*xmlPath)
	text := strings.ReplaceAll(html.UnescapeString(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(string(xmlSrc), "")), "\u00a0", " ")
	prod := func(num, name, next string) []rng {
		start := regexp.MustCompile(`\[` + num + `\]\s*` + name + `\s*::=`).FindStringIndex(text)
		if start == nil {
			log.Fatalf("production [%s] %s not found", num, name)
		}
		rest := text[start[1]:]
		end := strings.Index(rest, next)
		if end < 0 {
			log.Fatalf("end of production %s not found", name)
		}
		body := rest[:end]
		var rs []rng
		for _, m := range regexp.MustCompile(`\[#x([0-9A-Fa-f]+)-#x([0-9A-Fa-f]+)\]|#x([0-9A-Fa-f]+)`).FindAllStringSubmatch(body, -1) {
			if m[1] != "" {
				rs = append(rs, rng{hex16(m[1]), hex16(m[2])})
			} else {
				c := hex16(m[3])
				rs = append(rs, rng{c, c})
			}
		}
		if len(rs) == 0 {
			log.Fatalf("production %s is empty", name)
		}
		return normalize(rs)
	}
	baseChar := prod("85", "BaseChar", "[86]")
	ideographic := prod("86", "Ideographic", "[87]")
	combining := prod("87", "CombiningChar", "[88]")
	digit := prod("88", "Digit", "[89]")
	extender := prod("89", "Extender", "The character classes")

	letter := normalize(append(append([]rng{}, baseChar...), ideographic...))
	// \i: Letter | '_' | ':'
	nameStart := normalize(append(append([]rng{}, letter...), rng{'_', '_'}, rng{':', ':'}))
	// \c: NameChar ::= Letter | Digit | '.' | '-' | '_' | ':' | CombiningChar | Extender
	nameChar := append(append(append(append([]rng{}, letter...), digit...), combining...), extender...)
	nameChar = normalize(append(nameChar, rng{'.', '.'}, rng{'-', '-'}, rng{'_', '_'}, rng{':', ':'}))

	type block struct {
		name   string
		ranges []rng
	}
	var blocks []block
	index := map[string]int{}
	for _, line := range strings.Split(string(read(*blocksPath)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		codes, name, ok := strings.Cut(line, ";")
		if !ok {
			log.Fatalf("bad Blocks.txt line %q", line)
		}
		lo, hi, ok := strings.Cut(strings.TrimSpace(codes), "..")
		if !ok {
			log.Fatalf("bad range %q", codes)
		}
		name = strings.Join(strings.Fields(name), "")
		if strings.Contains(name, "Surrogates") {
			continue // F.1: bloki surogatów są wyłączone
		}
		r := rng{hex16(lo), hex16(hi)}
		if name == "PrivateUse" && r.lo > 0xFFFF {
			// Tablica z dodatku F ma PrivateUse tylko w BMP (E000–F8FF); obszary prywatne spoza BMP od Unicode
			// 4.0 nazywają się SupplementaryPrivateUseArea-A/B. Zestaw W3C (MS-Regex reL99, reM98, reN99) tego
			// oczekuje.
			continue
		}
		if i, ok := index[name]; ok {
			blocks[i].ranges = append(blocks[i].ranges, r)
			continue
		}
		index[name] = len(blocks)
		blocks = append(blocks, block{name, []rng{r}})
	}

	if *xsd2Path != "" {
		// Kontrola: każdy wiersz tablicy z dodatku F części 2 jest w Blocks.txt 3.1.
		src := string(read(*xsd2Path))
		i := strings.Index(src, "Start Code")
		j := strings.Index(src, "The blocks mentioned above exclude")
		cells := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`).FindAllStringSubmatch(src[i:j], -1)
		var vals []string
		for _, c := range cells {
			v := strings.TrimSpace(html.UnescapeString(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(c[1], "")))
			if v != "" {
				vals = append(vals, v)
			}
		}
		checked := 0
		for k := 0; k+2 < len(vals); k++ {
			if strings.HasPrefix(vals[k], "#x") && strings.HasPrefix(vals[k+1], "#x") {
				lo, hi, name := hex16(vals[k][2:]), hex16(vals[k+1][2:]), vals[k+2]
				b, ok := index[name]
				found := false
				if ok {
					for _, r := range blocks[b].ranges {
						if r.lo == lo && r.hi == hi {
							found = true
						}
					}
				}
				if !found {
					log.Fatalf("XSD Part 2 block %s %X-%X not in Blocks.txt", name, lo, hi)
				}
				checked++
				k += 2
			}
		}
		log.Printf("checked %d block rows of XSD Part 2 appendix F", checked)
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by gentables.go; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "// Źródła:\n//   - XML 1.0 Second Edition, dodatek B, https://www.w3.org/TR/2000/REC-xml-20001006 (SHA-256 %s)\n", sum(xmlSrc))
	fmt.Fprintf(&b, "//   - Unicode 3.1 Blocks.txt, https://www.unicode.org/Public/3.1-Update/Blocks-4.txt (SHA-256 %s)\n\n", sum(read(*blocksPath)))
	b.WriteString("package regex\n\n")
	emit := func(name, doc string, rs []rng) {
		fmt.Fprintf(&b, "// %s\nvar %s = rangeSet{", doc, name)
		for i, r := range rs {
			if i%6 == 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "{0x%04X, 0x%04X}, ", r.lo, r.hi)
		}
		b.WriteString("\n}\n\n")
	}
	emit("xmlNameStart", "xmlNameStart to \\i: Letter | '_' | ':' (XML 1.0 2e, produkcje [84]–[86]).", nameStart)
	emit("xmlNameChar", "xmlNameChar to \\c: NameChar (XML 1.0 2e, produkcja [4]).", nameChar)
	b.WriteString("// unicodeBlocks to bloki Unicode 3.1 dla \\p{IsX}, nazwy bez białych znaków (część 2, F.1.1).\n")
	b.WriteString("var unicodeBlocks = map[string]rangeSet{\n")
	names := make([]string, 0, len(blocks))
	for _, bl := range blocks {
		names = append(names, bl.name)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "%q: {", n)
		for _, r := range normalize(blocks[index[n]].ranges) {
			fmt.Fprintf(&b, "{0x%04X, 0x%04X}, ", r.lo, r.hi)
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("\\i %d ranges, \\c %d ranges, %d blocks", len(nameStart), len(nameChar), len(blocks))
}

func read(p string) []byte {
	if p == "" {
		log.Fatal("missing input file")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		log.Fatal(err)
	}
	return b
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func hex16(s string) rune {
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		log.Fatal(err)
	}
	return rune(v)
}

func normalize(rs []rng) []rng {
	sort.Slice(rs, func(i, j int) bool { return rs[i].lo < rs[j].lo })
	var out []rng
	for _, r := range rs {
		if n := len(out); n > 0 && r.lo <= out[n-1].hi+1 {
			if r.hi > out[n-1].hi {
				out[n-1].hi = r.hi
			}
			continue
		}
		out = append(out, r)
	}
	return out
}
