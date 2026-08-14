// Package participle enforces TS-N14: exported identifiers do not end in a
// present participle.
package participle

import (
	"fmt"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/words"
)

// Analyzer enforces TS-N14: exported identifiers do not end in a present participle.
var Analyzer = newAnalyzer()

var allowFlag string

func newAnalyzer() *analysis.Analyzer {
	built := &analysis.Analyzer{
		Name: "participle",
		Doc:  "TS-N14: exported identifiers do not end in a present participle.",
		Run:  run,
	}
	built.Flags.StringVar(&allowFlag, "allow", "",
		"comma-separated lowercase tokens extending the default noun allowlist")
	return built
}

// minParticiple is the shortest last-token length TS-N14 considers: below
// this, the built-in noun allowlist (ring, king, wing) would apply anyway.
const minParticiple = 4

// nounAllowlist is the default set of genuine nouns that happen to end in
// "-ing". "finding" (what an analyzer reports) and "blocking" (the
// specification's own severity vocabulary) entered the list when this
// repository's dogfood run flagged them — a false positive on a blocking
// rule is fixed in the analyzer, per correctness constraint 7.
var nounAllowlist = map[string]bool{
	"string": true, "encoding": true, "logging": true, "padding": true,
	"warning": true, "ring": true, "thing": true, "spring": true,
	"sibling": true, "setting": true, "king": true, "wing": true,
	"swing": true, "finding": true, "blocking": true,
}

func run(pass *analysis.Pass) (any, error) {
	allowed := allowlist()
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			checkDecl(pass, decl, allowed)
		}
	}
	return nil, nil
}

// allowlist parses the -allow flag into a set of lowercase tokens that
// extend nounAllowlist.
func allowlist() map[string]bool {
	extra := map[string]bool{}
	for _, token := range strings.Split(allowFlag, ",") {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" {
			extra[token] = true
		}
	}
	return extra
}

// checkDecl walks one top-level declaration for the identifiers TS-N14
// covers: func/method names, type names, const/var names, and struct
// fields (at any depth inside the declared type).
func checkDecl(pass *analysis.Pass, decl ast.Decl, allowed map[string]bool) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		check(pass, d.Name, allowed)
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			checkSpec(pass, spec, allowed)
		}
	}
}

// checkSpec walks one GenDecl spec: a type name plus any struct fields its
// type declares, or a const/var name plus any struct fields an anonymous
// struct type declares.
func checkSpec(pass *analysis.Pass, spec ast.Spec, allowed map[string]bool) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		check(pass, s.Name, allowed)
		checkFields(pass, s.Type, allowed)
	case *ast.ValueSpec:
		for _, name := range s.Names {
			check(pass, name, allowed)
		}
		if s.Type != nil {
			checkFields(pass, s.Type, allowed)
		}
	}
}

// checkFields walks expr for struct field names at any depth, covering
// both a named struct type and an anonymous struct type embedded in a
// var or const declaration.
func checkFields(pass *analysis.Pass, expr ast.Expr, allowed map[string]bool) {
	ast.Inspect(expr, func(node ast.Node) bool {
		structType, ok := node.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structType.Fields.List {
			for _, name := range field.Names {
				check(pass, name, allowed)
			}
		}
		return true
	})
}

// check reports TS-N14 when ident is exported and its last words.Split
// token is a non-noun participle.
func check(pass *analysis.Pass, ident *ast.Ident, allowed map[string]bool) {
	if !ident.IsExported() {
		return
	}
	tokens := words.Split(ident.Name)
	if len(tokens) == 0 {
		return
	}
	last := tokens[len(tokens)-1]
	lower := strings.ToLower(last)
	if len(lower) <= minParticiple || !strings.HasSuffix(lower, "ing") {
		return
	}
	if nounAllowlist[lower] || allowed[lower] {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      ident.Pos(),
		Category: "TS-N14",
		Message: fmt.Sprintf(
			"TS-N14: %q ends in the participle %q — a trailing participle cannot head a "+
				"design doc and derived names do not compose; prefer the noun (or add "+
				"-allow=%s for a genuine noun)", ident.Name, last, lower),
	})
}
