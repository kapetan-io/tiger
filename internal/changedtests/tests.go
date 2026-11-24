package changedtests

import (
	"fmt"
	"go/ast"
	"go/doc"
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

type testFunctions map[testName]definitions

type testName struct {
	dir      string
	function string
}

type definitions []definition

type definition struct {
	pkg       string
	file      string
	tokenText string
}

var testPrefixes = []string{"Test", "Benchmark", "Fuzz", "Example"}

type revisionTests struct {
	tests   testFunctions
	modules module.Modules
}

type runDir struct {
	path      string
	enclosing bool
}

func readTests(run runDir) (revisionTests, error) {
	dir := run.path
	modules, err := module.ReadModules(dir)
	if err == nil && run.enclosing {
		modules, err = modules.Enclosed(dir)
	}
	if err != nil {
		return revisionTests{}, err
	}
	read := revisionTests{tests: testFunctions{}, modules: modules}
	for _, mod := range modules.Read {
		files, err := module.GoFiles(filepath.Join(dir, filepath.FromSlash(mod.Dir)))
		if err != nil {
			return revisionTests{}, err
		}
		for _, pkgDir := range slices.Sorted(maps.Keys(files)) {
			pkg := path.Join(string(mod.ImportPath), pkgDir)
			for _, rel := range files[pkgDir] {
				if !strings.HasSuffix(rel, "_test.go") {
					continue
				}
				file := testFile{dir: dir, rel: path.Join(mod.Dir, rel)}
				if err := read.tests.read(file, pkg); err != nil {
					return revisionTests{}, err
				}
			}
		}
	}
	return read, nil
}

type testFile struct {
	dir string
	rel string
}

func (t testFunctions) read(file testFile, pkg string) error {
	src, err := os.ReadFile(filepath.Join(file.dir, filepath.FromSlash(file.rel)))
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.rel, src,
		parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return fmt.Errorf("cannot parse %w", err)
	}
	outputs := map[string]string{}
	for _, example := range doc.Examples(parsed) {
		outputs["Example"+example.Name] = fmt.Sprintf("output %t %q",
			example.Unordered, example.Output)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !isTestName(fn.Name.Name) {
			continue
		}
		body := src[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset]
		name := testName{dir: path.Dir(file.rel), function: fn.Name.Name}
		t[name] = append(t[name], definition{
			pkg:       pkg,
			file:      file.rel,
			tokenText: tokenText(body) + outputs[fn.Name.Name],
		})
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

func tokenText(src []byte) string {
	fset := token.NewFileSet()
	var scan scanner.Scanner
	scan.Init(fset.AddFile("", -1, len(src)), src, nil, 0)
	var out strings.Builder
	separator := ""
	maxTokens := len(src) + 2
	for range maxTokens {
		_, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON || tok == token.COMMA {
			separator = tok.String() + "\n"
			continue
		}
		if tok != token.RBRACE && tok != token.RPAREN && tok != token.RBRACK {
			out.WriteString(separator)
		}
		separator = ""
		out.WriteString(tok.String() + " " + lit + "\n")
	}
	return out.String()
}

type candidate struct {
	base definition
	head definition
}

type match func(candidate) bool

func sameFileAndTokens(c candidate) bool {
	return c.base.file == c.head.file && c.base.tokenText == c.head.tokenText
}

func sameTokens(c candidate) bool {
	return c.base.tokenText == c.head.tokenText
}

func sameFile(c candidate) bool {
	return c.base.file == c.head.file
}

func anyDefinition(candidate) bool {
	return true
}

type matching struct {
	matchedHead definitions
	base        definitions
	head        definitions
}

func (m matching) next(matches match) matching {
	out := matching{head: slices.Clone(m.head)}
	for _, base := range m.base {
		index := slices.IndexFunc(out.head, func(head definition) bool {
			return matches(candidate{base: base, head: head})
		})
		if index < 0 {
			out.base = append(out.base, base)
			continue
		}
		out.matchedHead = append(out.matchedHead, out.head[index])
		out.head = slices.Delete(out.head, index, index+1)
	}
	return out
}

func (t testFunctions) count() int {
	n := 0
	for _, name := range slices.SortedFunc(maps.Keys(t), testName.order) {
		n += len(t[name])
	}
	return n
}

func (t testFunctions) packages() int {
	seen := map[string]bool{}
	for _, name := range slices.SortedFunc(maps.Keys(t), testName.order) {
		for _, def := range t[name] {
			seen[def.pkg] = true
		}
	}
	return len(seen)
}

func (t testFunctions) outside(unread []module.Unread, roots []string) testFunctions {
	kept := testFunctions{}
	for _, name := range slices.SortedFunc(maps.Keys(t), testName.order) {
		owner := name.module(roots)
		if !slices.ContainsFunc(unread, func(mod module.Unread) bool { return mod.Dir == owner }) {
			kept[name] = t[name]
		}
	}
	return kept
}

func (n testName) module(roots []string) string {
	owner := ""
	for _, root := range roots {
		within := root == "." || n.dir == root || strings.HasPrefix(n.dir, root+"/")
		if within && len(root) > len(owner) {
			owner = root
		}
	}
	return owner
}
