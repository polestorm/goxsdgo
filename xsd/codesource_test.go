package xsd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// codePackages to pakiety goxsdgo, których kody błędów mogą dotrzeć do wywołującego: jako Error.Code albo w treści
// błędu zwracanego drugim wynikiem (limity, konstrukcje nieobsługiwane). Kody czytnika pochodzą z goxmlgo/xmlreader
// (readerCodes); jego stałe są znane przy rozwiązywaniu odwołań xmlreader.CodeX w tych pakietach.
var codePackages = []string{"datatypes", "regex", "schema", "validate"}

// readerCodes zwraca kody błędów eksportowane przez goxmlgo/xmlreader w wersji z go.mod (stała Code* → wartość),
// czytane z kodu źródłowego modułu w pamięci modułów.
func readerCodes(t *testing.T) map[string]string {
	t.Helper()
	dir, _ := goxmlgo(t)
	matches, err := filepath.Glob(filepath.Join(dir, "xmlreader", "*.go"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("goxmlgo/xmlreader sources in %s: %v", dir, err)
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, m := range matches {
		if strings.HasSuffix(m, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, m, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for name, v := range codeConsts(f) {
			out[name] = v
		}
	}
	return out
}

// codeConsts zwraca stałe Code* pliku z wartością będącą literałem napisu.
func codeConsts(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			vs := s.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if !strings.HasPrefix(n.Name, "Code") || i >= len(vs.Values) {
					continue
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					v, _ := strconv.Unquote(lit.Value)
					out[n.Name] = v
				}
			}
		}
	}
	return out
}

// sourceCodes czyta kod źródłowy pakietów (bez testów) i zwraca wszystkie kody błędów, które mogą zostać
// wyemitowane: wartości stałych Code* użytych w kodzie oraz złożenia „Code* + "punkt"”. Stała użyta wyłącznie
// jako przedrostek złożenia nie liczy się osobno.
func sourceCodes(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	type pkgFiles struct {
		name  string
		files []*ast.File
	}
	var pkgs []pkgFiles
	consts := map[string]map[string]string{"xmlreader": readerCodes(t)} // pakiet → nazwa stałej → wartość
	for _, p := range codePackages {
		dir := filepath.Join("..", "internal", p)
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		pf := pkgFiles{name: p}
		consts[p] = map[string]string{}
		for _, m := range matches {
			if strings.HasSuffix(m, "_test.go") || filepath.Base(m) == "gentables.go" {
				continue
			}
			f, err := parser.ParseFile(fset, m, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			pf.files = append(pf.files, f)
			for name, v := range codeConsts(f) {
				consts[p][name] = v
			}
		}
		pkgs = append(pkgs, pf)
	}
	// resolve zwraca wartość stałej wskazanej wyrażeniem (CodeX albo pakiet.CodeX) w pakiecie p.
	resolve := func(p string, e ast.Expr) (string, bool) {
		switch x := e.(type) {
		case *ast.Ident:
			v, ok := consts[p][x.Name]
			return v, ok
		case *ast.SelectorExpr:
			if pkg, ok := x.X.(*ast.Ident); ok {
				if m, ok := consts[pkg.Name]; ok {
					v, ok := m[x.Sel.Name]
					return v, ok
				}
			}
		}
		return "", false
	}
	out := map[string]string{} // kod → pierwsze miejsce użycia
	var dynamic []string
	for _, pf := range pkgs {
		for _, f := range pf.files {
			// Deklaracje stałych pomijamy: liczy się użycie, nie definicja.
			var inConst []ast.Node
			for _, d := range f.Decls {
				if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.CONST {
					inConst = append(inConst, g)
				}
			}
			isConstDecl := func(n ast.Node) bool {
				for _, c := range inConst {
					if n.Pos() >= c.Pos() && n.End() <= c.End() {
						return true
					}
				}
				return false
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if n == nil || isConstDecl(n) {
					return n == nil || !isConstDecl(n)
				}
				switch x := n.(type) {
				case *ast.BinaryExpr:
					if x.Op != token.ADD {
						return true
					}
					base, ok := resolve(pf.name, x.X)
					lit, isLit := x.Y.(*ast.BasicLit)
					suffix := ""
					if isLit && lit.Kind == token.STRING {
						suffix, _ = strconv.Unquote(lit.Value)
					}
					if ok && isLit && lit.Kind == token.STRING {
						code := base + suffix
						if _, seen := out[code]; !seen {
							out[code] = fset.Position(x.Pos()).String()
						}
						return false
					}
					// Numer punktu doklejony do zmiennej albo zmienna doklejona do stałej: kodu nie da się ustalić
					// z kodu źródłowego, więc katalogu nie da się sprawdzić. Takie miejsce jest błędem testu.
					if ok || (strings.HasPrefix(suffix, ".") && len(suffix) > 1 && suffix[1] >= '0' && suffix[1] <= '9') {
						dynamic = append(dynamic, fset.Position(x.Pos()).String())
					}
				case *ast.Ident, *ast.SelectorExpr:
					if v, ok := resolve(pf.name, x.(ast.Expr)); ok {
						if _, seen := out[v]; !seen {
							out[v] = fset.Position(x.Pos()).String()
						}
						return false
					}
				}
				return true
			})
		}
	}
	for _, d := range dynamic {
		t.Errorf("error code composed at run time at %s: build it from Code* constants and literals only, "+
			"so that the catalog can be checked against the source", d)
	}
	return out
}
