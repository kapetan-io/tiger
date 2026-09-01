// Package restrict reads a package's //tiger:restrict declaration.
//
// The declaration is an intent directive in the package doc comment (the
// specification's TS-P01 form), so it is read from each file's Doc group,
// never through the pin-scoped pins.Collect. Both restrictions (TS-P01,
// TS-P02) and closedworld (TS-K03) read it through this one helper, so a
// declaration can never mean one thing to one analyzer and another to the
// next. Malformed declarations are skipped here; directives owns their
// TS-L09 findings.
package restrict

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/directive"
)

// Declaration is one well-formed //tiger:restrict in a package doc comment.
type Declaration struct {
	Restriction directive.Restriction
	// Pos locates the directive comment.
	Pos token.Pos
	// File is the file whose doc comment carries the directive; its package
	// clause is where TS-P02 lines are reported.
	File *ast.File
}

// Collect returns every well-formed restrict declaration in the package's
// doc comments, in file order. A package makes a claim only through the
// first; restrictions reports any second one.
func Collect(pass *analysis.Pass) []Declaration {
	declared := []Declaration{}
	for _, file := range pass.Files {
		if file.Doc == nil {
			continue
		}
		for _, comment := range file.Doc.List {
			if !directive.Is(comment.Text) {
				continue
			}
			parsed, err := directive.Parse(comment.Text)
			if err != nil || parsed.Verb != "restrict" {
				continue
			}
			restriction, err := directive.ParseRestrict(parsed.Args)
			if err != nil {
				continue
			}
			declared = append(declared, Declaration{
				Restriction: restriction, Pos: comment.Pos(), File: file,
			})
		}
	}
	return declared
}

// Declared returns the package's claim: the first well-formed declaration,
// or false when the package makes none.
func Declared(pass *analysis.Pass) (Declaration, bool) {
	declared := Collect(pass)
	if len(declared) == 0 {
		return Declaration{}, false
	}
	return declared[0], true
}
