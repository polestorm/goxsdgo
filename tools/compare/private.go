package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/internal/xmlreader"
	"github.com/polestorm/goxsdgo/xsd"
)

// runPrivate porównuje goxsdgo i libxml2 na prywatnym korpusie z katalogu GOXSDGO_PRIVATE_CORPUS (K3, K7, ADR 0011):
// czas walidacji strumieniowej, werdykt i wiersz pierwszego błędu. Schemat każdego pliku wybiera przestrzeń nazw
// elementu głównego, według głównych schematów MF z manifestu.
//
// Dane prywatne (testdata/README.md): polecenie nie wypisuje ani nie zapisuje nazw, ścieżek, wartości ani fragmentów
// plików — tylko liczby zbiorcze i kody błędów. Bez zmiennej GOXSDGO_PRIVATE_CORPUS nie robi nic.
func runPrivate(root string) error {
	dir := os.Getenv("GOXSDGO_PRIVATE_CORPUS")
	if dir == "" {
		fmt.Println("GOXSDGO_PRIVATE_CORPUS not set; nothing to do")
		return nil
	}
	src, err := testkit.LoadSources(root)
	if err != nil {
		return err
	}
	schemas, err := filepath.Abs(filepath.Join(root, "testdata", "schemas", "mf"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(schemas); err != nil {
		return errors.New(testkit.FetchHint)
	}
	cleanup, err := loadCatalog(root, "mf-schemas", schemas)
	if err != nil {
		return err
	}
	defer cleanup()
	byNS := map[string]string{}
	for _, ds := range src.Sets {
		if ds.Name != "mf-schemas" {
			continue
		}
		for _, f := range ds.Files {
			if f.Namespace != "" && !strings.Contains(f.Path, "/") {
				if _, err := os.Stat(filepath.Join(schemas, f.Path)); err == nil {
					byNS[f.Namespace] = f.Path
				}
			}
		}
	}
	var files []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".xml") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)

	gcfg := xsd.Config{Locations: src.Locations("mf-schemas")}
	gsets := map[string]*xsd.Set{}
	lsets := map[string]*libSchema{}
	defer func() {
		for _, s := range lsets {
			s.Close()
		}
	}()
	var (
		noSchema, unreadable, compared, sameVerdict, bothInvalid, sameLine int
		gLimit, gUnsupported, lFail                                        int
		totalBytes, comparedBytes                                          int64
		gTime, lTime                                                       time.Duration
		biggest                                                            int64
		bigG, bigL                                                         time.Duration
	)
	pairs := map[string]int{}
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			unreadable++
			continue
		}
		totalBytes += info.Size()
		ns, err := rootNamespace(file)
		if err != nil {
			unreadable++
			continue
		}
		schema, ok := byNS[ns]
		if !ok {
			noSchema++
			continue
		}
		gs, ok := gsets[schema]
		if !ok {
			gs, err = gcfg.Compile(os.DirFS(schemas), schema)
			if err != nil {
				return fmt.Errorf("goxsdgo: MF schema does not compile: %v", err)
			}
			gsets[schema] = gs
		}
		ls, ok := lsets[schema]
		if !ok {
			ls, err = parseSchemaFile(filepath.Join(schemas, filepath.FromSlash(schema)))
			if err != nil {
				return fmt.Errorf("libxml2: MF schema does not compile: %v", err)
			}
			lsets[schema] = ls
		}
		start := time.Now()
		gerrs, gerr := goxsdgoErrors(gs, file)
		gd := time.Since(start)
		start = time.Now()
		lerrs, lerr := ls.ValidateFile(file, 1000)
		ld := time.Since(start)
		switch {
		case errors.Is(gerr, xsd.ErrLimit):
			gLimit++
			continue
		case errors.Is(gerr, errors.ErrUnsupported):
			gUnsupported++
			continue
		case gerr != nil:
			unreadable++
			continue
		case lerr != nil && !errors.Is(lerr, errTooManyErrors):
			lFail++
			continue
		}
		compared++
		comparedBytes += info.Size()
		gTime += gd
		lTime += ld
		if info.Size() > biggest {
			biggest, bigG, bigL = info.Size(), gd, ld
		}
		gValid, lValid := len(gerrs) == 0, !hasErrors(lerrs)
		if gValid == lValid {
			sameVerdict++
		} else {
			code := "valid"
			if !gValid {
				code = gerrs[0].Code
			}
			lcode := "valid"
			if !lValid {
				lcode = fmt.Sprint(firstError(lerrs).Code)
			}
			pairs["verdict: goxsdgo "+code+" / libxml2 "+lcode]++
		}
		if !gValid && !lValid {
			bothInvalid++
			if gerrs[0].Line == firstError(lerrs).Line {
				sameLine++
			} else {
				pairs[fmt.Sprintf("first line: goxsdgo %s / libxml2 %d", gerrs[0].Code, firstError(lerrs).Code)]++
			}
		}
	}
	mb := func(n int64) float64 { return float64(n) / (1 << 20) }
	fmt.Printf("private corpus: %d XML files, %.1f MB; without a schema in the corpus: %d; unreadable or not XML: %d\n",
		len(files), mb(totalBytes), noSchema, unreadable)
	fmt.Printf("goxsdgo stopped by a library limit: %d, unsupported construct: %d; libxml2 failed to run: %d\n",
		gLimit, gUnsupported, lFail)
	fmt.Printf("compared: %d files, %.1f MB\n", compared, mb(comparedBytes))
	if compared > 0 {
		fmt.Printf("time: goxsdgo %v, libxml2 %v, ratio %.2f×\n", gTime.Round(time.Millisecond),
			lTime.Round(time.Millisecond), gTime.Seconds()/max(lTime.Seconds(), 1e-9))
		fmt.Printf("largest compared file: %.1f MB, goxsdgo %v, libxml2 %v, ratio %.2f×\n", mb(biggest),
			bigG.Round(time.Millisecond), bigL.Round(time.Millisecond), bigG.Seconds()/max(bigL.Seconds(), 1e-9))
		fmt.Printf("same verdict: %d/%d (%.2f%%); same first error line (both invalid): %d/%d\n", sameVerdict, compared,
			100*float64(sameVerdict)/float64(compared), sameLine, bothInvalid)
	}
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %4d  %s\n", pairs[k], k)
	}
	return nil
}

// rootNamespace zwraca przestrzeń nazw elementu głównego pliku.
func rootNamespace(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := xmlreader.New(f, xmlreader.Options{})
	for {
		tok, err := r.Next()
		if err == io.EOF {
			return "", errors.New("no root element")
		}
		if err != nil {
			return "", err
		}
		if tok.Kind == xmlreader.KindStartElement {
			return tok.Name.Space, nil
		}
	}
}

// runTimeFile mierzy czas walidacji strumieniowej jednego pliku przez goxsdgo i przez libxml2 (K7), z najlepszego
// z runs przebiegów każdego walidatora. schema to ścieżka głównego schematu MF względem testdata/schemas/mf.
func runTimeFile(root, file, schema string, runs int) error {
	src, err := testkit.LoadSources(root)
	if err != nil {
		return err
	}
	schemas, err := filepath.Abs(filepath.Join(root, "testdata", "schemas", "mf"))
	if err != nil {
		return err
	}
	cleanup, err := loadCatalog(root, "mf-schemas", schemas)
	if err != nil {
		return err
	}
	defer cleanup()
	gs, err := xsd.Config{Locations: src.Locations("mf-schemas")}.Compile(os.DirFS(schemas), schema)
	if err != nil {
		return err
	}
	ls, err := parseSchemaFile(filepath.Join(schemas, filepath.FromSlash(schema)))
	if err != nil {
		return err
	}
	defer ls.Close()
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	best := func(f func() error) (time.Duration, error) {
		b := time.Duration(1 << 62)
		for i := 0; i < runs; i++ {
			start := time.Now()
			if err := f(); err != nil {
				return 0, err
			}
			b = min(b, time.Since(start))
		}
		return b, nil
	}
	gd, err := best(func() error {
		errs, err := goxsdgoErrors(gs, file)
		if err == nil && len(errs) > 0 {
			err = fmt.Errorf("goxsdgo: %d errors, first %v", len(errs), errs[0])
		}
		return err
	})
	if err != nil {
		return err
	}
	ld, err := best(func() error {
		errs, err := ls.ValidateFile(file, 1000)
		if err == nil && hasErrors(errs) {
			err = fmt.Errorf("libxml2: %d errors", len(errs))
		}
		return err
	})
	if err != nil {
		return err
	}
	mb := float64(info.Size()) / (1 << 20)
	fmt.Printf("%.1f MB: goxsdgo %v (%.1f MB/s), libxml2 streaming %v (%.1f MB/s), ratio %.2f× (best of %d)\n",
		mb, gd.Round(time.Millisecond), mb/gd.Seconds(), ld.Round(time.Millisecond), mb/ld.Seconds(),
		gd.Seconds()/ld.Seconds(), runs)
	return nil
}
