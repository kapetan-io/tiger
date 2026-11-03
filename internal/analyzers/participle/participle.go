// Package participle enforces TS-N14: exported identifiers do not end in a
// present participle.
package participle

import (
	"fmt"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/words"
	"github.com/kapetan-io/tiger/internal/config"
)

var Analyzer = newAnalyzer()

var allowFlag string

func newAnalyzer() *analysis.Analyzer {
	built := &analysis.Analyzer{
		Name: "participle",
		Doc:  "TS-N14: exported identifiers do not end in a present participle.",
		Run:  run,
	}
	built.Flags.StringVar(&allowFlag, "allow", "",
		"comma-separated lowercase tokens extending the default noun allowlist "+
			"(or participle.allow entries in tiger.yaml)")
	return built
}

const minParticiple = 4

var nounAllowlist = map[string]bool{
	"string": true, "encoding": true, "logging": true, "padding": true,
	"warning": true, "ring": true, "thing": true, "spring": true,
	"sibling": true, "setting": true, "king": true, "wing": true,
	"swing": true, "finding": true, "blocking": true, "binding": true,
	"waiting": true, "nothing": true, "something": true, "anything": true,
	"everything": true,
}

func run(pass *analysis.Pass) (any, error) {
	allowed := allowlist(pass)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			checkDecl(pass, decl, allowed)
		}
	}
	return nil, nil
}

func allowlist(pass *analysis.Pass) map[string]bool {
	extra := map[string]bool{}
	tokens := strings.Split(allowFlag, ",")
	tokens = append(tokens, config.Values(
		pass.Pkg.Path(), config.Key{Analyzer: "participle", Flag: "allow"},
	)...)
	for _, token := range tokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" {
			extra[token] = true
		}
	}
	return extra
}

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
			"TS-N14: %q ends in the -ing word %q, which names an activity rather than a "+
				"thing — rename it to the noun it stands for, or pass the flag -allow=%s if "+
				"it really is a noun", ident.Name, last, lower),
	})
}
