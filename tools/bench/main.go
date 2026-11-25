// Command bench porównuje goxsdgo z innymi walidatorami XSD na tych samych danych: zestaw W3C (XSD 1.0 2e, ta sama
// lista przypadków i reguły zaliczenia co TestW3CSuite), korpus MF, rozmiar programu, czas i pamięć na
// syntetycznym JPK, próby bezpieczeństwa i obsługi konstrukcji. Wyniki trafiają do README (sekcja Comparison).
//
// Kandydaci są używani wyłącznie jako czarne skrzynki, przez publiczne API (docs/provenance.md, ADR 0002).
// Moduł jest osobny, żeby biblioteka goxsdgo nie miała zależności.
//
//	go -C tools/bench run . -root ../.. w3c        # zestaw W3C, wszyscy kandydaci (kilka minut)
//	go -C tools/bench run . -root ../.. corpus     # korpus MF
//	go -C tools/bench run . -root ../.. probe      # bezpieczeństwo, konstrukcje, błędy, wzorce
//	go -C tools/bench run . -root ../.. perf       # czas i pamięć (osobne procesy, /usr/bin/time)
//	go -C tools/bench run . -root ../.. size       # rozmiar programu i linie kodu
//
// Wyniki maszynowe: <root>/out/bench/*.json i *.tsv.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := flag.String("root", "../..", "goxsdgo repository root")
	only := flag.String("only", "", "comma-separated candidate names (default: all)")
	flag.Parse()
	// Ścieżki bezwzględne: część kandydatów wylicza z nich adresy bazowe dokumentów.
	if abs, err := filepath.Abs(*root); err == nil {
		*root = abs
	}
	out := filepath.Join(*root, "out", "bench")
	pick := func() []candidate {
		var list []candidate
		for _, c := range candidates() {
			if *only == "" || strings.Contains(","+*only+",", ","+c.name()+",") {
				list = append(list, c)
			}
		}
		return list
	}
	var err error
	switch flag.Arg(0) {
	case "w3c":
		var all []*w3cResult
		for _, c := range pick() {
			r, e := runW3C(*root, out, c)
			if e != nil {
				err = e
				break
			}
			printW3C(r)
			all = append(all, r)
		}
		if err == nil {
			err = writeJSON(out, "w3c.json", all)
		}
	case "corpus":
		var all []*corpusResult
		for _, c := range pick() {
			r, e := runCorpus(*root, c)
			if e != nil {
				err = e
				break
			}
			fmt.Printf("%-14s schemas %d / %d, documents %d / %d %s\n", r.Candidate, r.SchemasOK, r.Schemas,
				r.DocumentsOK, r.Documents, r.FirstFailure)
			all = append(all, r)
		}
		if err == nil {
			err = writeJSON(out, "corpus.json", all)
		}
	case "probe":
		err = runProbes(*root, out, pick())
	case "perf":
		err = runPerf(*root, out, pick(), flag.Args()[1:])
	case "perf-one":
		err = perfOne(*root, flag.Args()[1:])
	case "size":
		err = runSize(*root, out)
	default:
		fmt.Fprintln(os.Stderr, "usage: bench [-root DIR] [-only NAMES] w3c|corpus|probe|perf|size")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func writeJSON(dir, name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeOut(dir, name, string(b)+"\n")
}
