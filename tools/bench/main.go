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
//	go -C tools/bench build -o ../../out/bench/bench . && (cd tools/bench && ../../out/bench/bench -root ../.. -runs 5 perf 20 200)
//	                                                  # czas i pamięć (osobne procesy, mediana z -runs przebiegów)
//	go -C tools/bench run . -root ../.. size       # rozmiar programu i linie kodu
//	go -C tools/bench run . -root ../.. -docs contested  # przypadki sporne W3C i wyniki bez nich (po w3c)
//	go -C tools/bench run . -root ../.. publish    # surowe wyniki do docs/results (po pozostałych trybach)
//
// Wyniki maszynowe: <root>/out/bench/*.json i *.tsv, a opis maszyny i wersji w environment-<tryb>.json.
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
	docs := flag.Bool("docs", false, "contested: also write docs/contested-cases.md")
	runs := flag.Int("runs", 5, "perf: runs per candidate and size (median and range are reported)")
	flag.StringVar(&xmllint, "xmllint", "xmllint", "path to libxml2's xmllint (perf, probe)")
	flag.Parse()
	xmllint = resolveXmllint(xmllint)
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
	if m := flag.Arg(0); m != "" && m != "perf-one" && m != "publish" && m != "contested" {
		if e := writeEnv(*root, out, m); e != nil {
			fmt.Fprintln(os.Stderr, "bench: environment:", e)
		}
	}
	switch flag.Arg(0) {
	case "w3c":
		var all []*w3cResult
		list := pick()
		if *only == "" || strings.Contains(","+*only+",", ","+goxsdgo5Name+",") {
			list = append(list, goxsdgoCand{names5: true})
		}
		for _, c := range list {
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
		err = runPerf(*root, out, pick(), flag.Args()[1:], *runs)
	case "perf-one":
		err = perfOne(*root, flag.Args()[1:])
	case "size":
		err = runSize(*root, out)
	case "contested":
		err = runContested(*root, out, *docs)
	case "publish":
		err = runPublish(*root, out)
	default:
		fmt.Fprintln(os.Stderr, "usage: bench [-root DIR] [-only NAMES] [-docs] w3c|corpus|probe|perf|size|contested|publish")
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
