package main

// Adaptery kandydatów do porównania. Każdy kandydat jest używany wyłącznie przez publiczne API opisane w jego
// README i dokumentacji pakietu (pkg.go.dev); kodu źródłowego kandydatów nie czytamy (docs/provenance.md,
// ADR 0002). Tam, gdzie API było niejasne, rozstrzygał eksperyment czarnej skrzynki.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	cbsigma "github.com/cB-Sigma/xsd"
	jacoelho "github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
	"github.com/knroy/go-xml/v2/xdm"
	knroy "github.com/knroy/go-xml/v2/xsd"

	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/xsd"
)

// errNotComparable: kandydat nie ma sposobu, żeby wykonać przypadek (np. brak obsługi wskazówek
// xsi:schemaLocation); przypadek liczymy osobno, nie jako werdykt.
var errNotComparable = errors.New("not comparable")

// errUndecided: kandydat nie wydał werdyktu (limit, konstrukcja nieobsługiwana).
var errUndecided = errors.New("undecided")

// env opisuje, gdzie leżą schematy: katalog główny (zestaw W3C albo schematy MF) i przypisanie adresów
// bezwzględnych (http://…) do plików lokalnych, bo żaden kandydat nie sięga tu do sieci.
type env struct {
	dir  string            // katalog główny, ścieżki dokumentów są względem niego (z ukośnikami)
	urls map[string]string // adres → bezwzględna ścieżka pliku
}

func (e env) abs(p string) string { return filepath.Join(e.dir, filepath.FromSlash(p)) }

// realEnv rozwija dowiązania w ścieżkach: dane testowe bywają dowiązaniami (np. w kopii roboczej), a część
// kandydatów odrzuca plik, którego ścieżka po rozwinięciu wychodzi poza katalog główny.
func realEnv(e env) env {
	if d, err := filepath.EvalSymlinks(e.dir); err == nil {
		e.dir = d
	}
	urls := map[string]string{}
	for u, p := range e.urls {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		urls[u] = p
	}
	e.urls = urls
	return e
}

type candidate interface {
	name() string
	// compile kompiluje schemat z dokumentów docs; błąd znaczy „schemat niepoprawny”, chyba że opakowuje
	// errUndecided albo errNotComparable.
	compile(e env, docs []string) (compiled, error)
	// compileHints kompiluje schemat z dokumentów docs i wskazówek xsi:schemaLocation dokumentu instance;
	// kandydat bez wskazówek zwraca to samo co compile (gdy docs niepuste) albo errNotComparable.
	compileHints(e env, docs []string, instance string) (compiled, error)
}

type compiled interface {
	// validate zwraca true dla dokumentu poprawnego; błąd opakowujący errUndecided znaczy „bez werdyktu”.
	validate(path string) (bool, error)
}

// --- goxsdgo ---

type goxsdgoCand struct{}

func (goxsdgoCand) name() string { return "goxsdgo" }

func (goxsdgoCand) config(e env) (xsd.Config, fs.FS) {
	// Pliki spoza katalogu głównego (xml.xsd, xlink.xsd) podłączamy pod prefiksem, jak w TestW3CSuite.
	const extra = "_extra"
	loc := map[string]string{}
	extraDir := ""
	for u, p := range e.urls {
		if rel, err := filepath.Rel(e.dir, p); err == nil && !strings.HasPrefix(rel, "..") {
			loc[u] = filepath.ToSlash(rel)
			continue
		}
		extraDir = filepath.Dir(p)
		loc[u] = extra + "/" + filepath.Base(p)
	}
	fsys := os.DirFS(e.dir)
	if extraDir != "" {
		fsys = testkit.Overlay(fsys, extra, os.DirFS(extraDir))
	}
	return xsd.Config{Locations: loc}, fsys
}

type goxsdgoSet struct {
	set  *xsd.Set
	fsys fs.FS
	dir  string
}

func (c goxsdgoCand) compile(e env, docs []string) (compiled, error) {
	cfg, fsys := c.config(e)
	set, err := cfg.Compile(fsys, docs...)
	if errors.Is(err, errors.ErrUnsupported) {
		return nil, fmt.Errorf("%w: %v", errUndecided, err)
	}
	if err != nil {
		return nil, err
	}
	return goxsdgoSet{set, fsys, e.dir}, nil
}

func (c goxsdgoCand) compileHints(e env, docs []string, _ string) (compiled, error) {
	cfg, fsys := c.config(e)
	cfg.SchemaLocationHints = true
	set, err := cfg.Compile(fsys, docs...)
	if errors.Is(err, errors.ErrUnsupported) {
		return nil, fmt.Errorf("%w: %v", errUndecided, err)
	}
	if err != nil {
		return nil, err
	}
	return goxsdgoSet{set, fsys, e.dir}, nil
}

func (s goxsdgoSet) validate(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	name := path
	if rel, err := filepath.Rel(s.dir, path); err == nil && !strings.HasPrefix(rel, "..") {
		name = filepath.ToSlash(rel)
	}
	errs, err := s.set.ValidateDocument(f, name)
	switch {
	case errors.Is(err, xsd.ErrTooManyErrors):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("%w: %v", errUndecided, err)
	}
	return len(errs) == 0, nil
}

// --- github.com/cB-Sigma/xsd ---

type cbsigmaCand struct{}

func (cbsigmaCand) name() string { return "cB-Sigma/xsd" }

func (cbsigmaCand) opts(e env) cbsigma.LoadOptions {
	return cbsigma.LoadOptions{Fetch: func(u string) ([]byte, error) {
		if p, ok := e.urls[u]; ok {
			return os.ReadFile(p)
		}
		return nil, fmt.Errorf("no local copy of %s", u)
	}}
}

type cbsigmaSet struct{ s *cbsigma.Schema }

func (c cbsigmaCand) compile(e env, docs []string) (compiled, error) {
	s, err := cbsigma.LoadWith(os.DirFS(e.dir), c.opts(e), docs...)
	if err != nil {
		return nil, err
	}
	return cbsigmaSet{s}, nil
}

func (c cbsigmaCand) compileHints(e env, docs []string, instance string) (compiled, error) {
	doc, err := os.ReadFile(e.abs(instance))
	if err != nil {
		return nil, err
	}
	s, err := cbsigma.LoadHinted(os.DirFS(e.dir), cbsigma.HintOptions{LoadOptions: c.opts(e), DocPath: instance}, doc, docs...)
	if err != nil {
		return nil, err
	}
	return cbsigmaSet{s}, nil
}

func (s cbsigmaSet) validate(path string) (bool, error) {
	doc, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return s.s.Validate(doc) == nil, nil
}

// --- github.com/jacoelho/xsd ---

type jacoelhoCand struct{}

func (jacoelhoCand) name() string { return "jacoelho/xsd" }

type jacoelhoSet struct {
	e *jacoelho.Engine
	// maxBytes > 0 podnosi domyślny limit 64 MiB na dokument (tylko pomiar wydajności na dużym pliku).
	maxBytes int64
}

func (jacoelhoCand) compile(e env, docs []string) (compiled, error) {
	resolver := jacoelho.ResolverFunc(func(base, location string) (jacoelho.SchemaSource, error) {
		if p, ok := e.urls[location]; ok {
			return jacoelho.File(p), nil
		}
		return jacoelho.SchemaSource{}, xsderrors.ErrSchemaNotFound
	})
	var srcs []jacoelho.SchemaSource
	for _, d := range docs {
		srcs = append(srcs, jacoelho.File(e.abs(d)).WithResolver(resolver))
	}
	eng, err := jacoelho.Compile(srcs...)
	if err != nil {
		if xsderrors.IsUnsupported(err) {
			return nil, fmt.Errorf("%w: %v", errUndecided, err)
		}
		return nil, err
	}
	return jacoelhoSet{e: eng}, nil
}

func (c jacoelhoCand) compileHints(e env, docs []string, _ string) (compiled, error) {
	if len(docs) == 0 {
		return nil, errNotComparable // README: hints never supply components
	}
	return c.compile(e, docs)
}

func (s jacoelhoSet) validate(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if s.maxBytes > 0 {
		err = s.e.ValidateWithOptions(f, jacoelho.ValidateOptions{MaxInstanceBytes: s.maxBytes})
	} else {
		err = s.e.Validate(f)
	}
	switch {
	case err == nil:
		return true, nil
	case xsderrors.IsConclusiveValidation(err):
		return false, nil
	}
	return false, fmt.Errorf("%w: %v", errUndecided, err)
}

// --- github.com/knroy/go-xml/v2 ---

type knroyCand struct{}

func (knroyCand) name() string { return "knroy/go-xml" }

// knroyResolver: adresy znane z env z plików lokalnych, pozostałe przez RootedFileResolver.
type knroyResolver struct {
	urls  map[string]string
	files knroy.Resolver
}

func (r knroyResolver) Resolve(namespace, location, base string) (io.ReadCloser, string, error) {
	if p, ok := r.urls[location]; ok {
		f, err := os.Open(p)
		return f, location, err
	}
	return r.files.Resolve(namespace, location, base)
}

func (knroyCand) opts(e env) knroy.Options {
	roots := []string{e.dir}
	for _, p := range e.urls {
		roots = append(roots, filepath.Dir(p))
	}
	return knroy.Options{Resolver: knroyResolver{e.urls, knroy.RootedFileResolver(roots)}}
}

type knroySet struct {
	s    *knroy.Schema
	opts knroy.Options
	// hints: przed walidacją dołącz schematy wskazane przez dokument
	hints bool
}

func (c knroyCand) compile(e env, docs []string) (compiled, error) {
	var abs []string
	for _, d := range docs {
		abs = append(abs, e.abs(d))
	}
	o := c.opts(e)
	s, err := knroy.LoadFiles(abs, o)
	if err != nil {
		if errors.Is(err, xdm.ErrResourceLimit) {
			return nil, fmt.Errorf("%w: %v", errUndecided, err)
		}
		return nil, err
	}
	return knroySet{s: s, opts: o}, nil
}

func (c knroyCand) compileHints(e env, docs []string, _ string) (compiled, error) {
	if len(docs) == 0 {
		return knroySet{s: knroy.NewSchema(), opts: c.opts(e), hints: true}, nil
	}
	set, err := c.compile(e, docs)
	if err != nil {
		return nil, err
	}
	ks := set.(knroySet)
	ks.hints = true
	return ks, nil
}

func (s knroySet) validate(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	// Dokumenty zestawu W3C bywają z DOCTYPE; w porównaniu pozwalamy na niego (domyślnie kandydat go odrzuca),
	// próba bezpieczeństwa (probe) używa ustawień domyślnych.
	tree, err := knroy.ParseDocument(f, xdm.ParseOptions{AllowDOCTYPE: true, BaseURI: path, DocumentURI: path})
	f.Close()
	if err != nil {
		return false, nil // dokument źle uformowany według kandydata
	}
	schema := s.s
	if s.hints {
		policy := knroy.InstanceLocationPolicy{
			AllowNamespace:   func(string) bool { return true },
			AllowNoNamespace: true,
			Resolver:         s.opts.Resolver,
		}
		hinted, err := schema.WithInstanceLocations(tree.Root, policy, s.opts)
		if err != nil {
			if errors.Is(err, xdm.ErrResourceLimit) {
				return false, fmt.Errorf("%w: %v", errUndecided, err)
			}
			return false, nil
		}
		schema = hinted
	}
	err = schema.Validate(tree.Root, knroy.ValidateOptions{})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, xdm.ErrResourceLimit):
		return false, fmt.Errorf("%w: %v", errUndecided, err)
	}
	return false, nil
}

func candidates() []candidate {
	return []candidate{goxsdgoCand{}, knroyCand{}, jacoelhoCand{}, cbsigmaCand{}}
}
