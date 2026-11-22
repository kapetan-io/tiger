package changedtests

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kapetan-io/tiger/internal/module"
)

type Tests map[TestName]Definitions

type TestName struct {
	Package  string
	Function string
}

type Definitions []Definition

type Definition struct {
	File   string
	Tokens string
}

var testPrefixes = []string{"Test", "Benchmark", "Fuzz", "Example"}

func readTests(dir string) (Tests, error) {
	pkgPath, err := modulePath(dir)
	if err != nil {
		return nil, err
	}
	files, err := module.GoFiles(dir)
	if err != nil {
		return nil, err
	}
	tests := Tests{}
	for _, pkgDir := range slices.Sorted(maps.Keys(files)) {
		for _, rel := range files[pkgDir] {
			if !strings.HasSuffix(rel, "_test.go") {
				continue
			}
			err := tests.read(sourceFile{dir: dir, rel: rel}, path.Join(string(pkgPath), pkgDir))
			if err != nil {
				return nil, err
			}
		}
	}
	return tests, nil
}

func hasGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

func modulePath(dir string) (module.Path, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	dirs := []string{abs}
	for range strings.Count(abs, string(filepath.Separator)) {
		dirs = append(dirs, filepath.Dir(dirs[len(dirs)-1]))
	}
	found := slices.IndexFunc(dirs, hasGoMod)
	if found < 0 {
		return "", fmt.Errorf("no go.mod in %s or any directory above it", abs)
	}
	root, err := module.ReadPath(dirs[found])
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(dirs[found], abs)
	if err != nil {
		return "", err
	}
	return module.Path(path.Join(string(root), filepath.ToSlash(rel))), nil
}

type sourceFile struct {
	dir string
	rel string
}

func (t Tests) read(file sourceFile, pkg string) error {
	src, err := os.ReadFile(filepath.Join(file.dir, filepath.FromSlash(file.rel)))
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.rel, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("cannot parse %w", err)
	}
	if strings.HasSuffix(parsed.Name.Name, "_test") {
		pkg += "_test"
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !isTestName(fn.Name.Name) {
			continue
		}
		body := src[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset]
		name := TestName{Package: pkg, Function: fn.Name.Name}
		t[name] = append(t[name], Definition{File: file.rel, Tokens: tokens(body)})
	}
	return nil
}

func isTestName(name string) bool {
	for _, prefix := range testPrefixes {
		rest, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		next, _ := utf8.DecodeRuneInString(rest)
		return rest == "" || !unicode.IsLower(next)
	}
	return false
}

func tokens(src []byte) string {
	fset := token.NewFileSet()
	var scan scanner.Scanner
	scan.Init(fset.AddFile("", -1, len(src)), src, nil, 0)
	var out strings.Builder
	pending := ""
	for range len(src) + 2 {
		_, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON || tok == token.COMMA {
			pending = tok.String() + "\n"
			continue
		}
		if tok != token.RBRACE && tok != token.RPAREN && tok != token.RBRACK {
			out.WriteString(pending)
		}
		pending = ""
		out.WriteString(tok.String() + " " + lit + "\n")
	}
	return out.String()
}

func (d Definitions) same(other Definitions) bool {
	return slices.Equal(d.sortedTokens(), other.sortedTokens())
}

func (d Definitions) sortedTokens() []string {
	out := make([]string, len(d))
	for i, def := range d {
		out[i] = def.Tokens
	}
	slices.Sort(out)
	return out
}

func (t Tests) packages() int {
	var names []string
	for name := range t {
		names = append(names, name.Package)
	}
	slices.Sort(names)
	return len(slices.Compact(names))
}
