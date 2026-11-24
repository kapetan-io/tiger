// Package packagecomment enforces TS-L15 and TS-L16: a package comment says
// only why the package exists, and every production file carries code.
package packagecomment

import (
	"go/ast"
	"go/doc/comment"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/config"
)

var Analyzer = &analysis.Analyzer{
	Name: "packagecomment",
	Doc: "TS-L15: a package comment is one paragraph. " +
		"TS-L16: no production file holds only comments and the package clause.",
	Run: run,
}

const (
	longMessage = "TS-L15: this package comment runs past one paragraph — " +
		"tiger expects it to say only why the package exists — " +
		"keep the first paragraph and move the rest into a test, or delete it"
	splitMessage = "TS-L15: this is one of several files with a package comment, " +
		"which go doc joins into one — tiger expects one paragraph in one file — " +
		"keep the package comment in one file and delete it from the others"
	commentOnlyMessage = "TS-L16: this file holds only comments and the package clause — " +
		"tiger expects every production file to carry code — " +
		"move any package comment above the package clause of a file with code, " +
		"then delete this file"
)

func run(pass *analysis.Pass) (any, error) {
	if len(config.Values(pass.Pkg.Path(), config.Key{Analyzer: "comments", Flag: "allow"})) > 0 {
		return nil, nil
	}
	docs := []*ast.CommentGroup{}
	count := 0
	for _, file := range pass.Files {
		if exempt(pass, file) {
			continue
		}
		if found := blocks(file.Doc); found > 0 {
			docs = append(docs, file.Doc)
			count += found
		}
		if len(file.Decls) == 0 && len(file.Comments) > 0 {
			pass.Report(analysis.Diagnostic{
				Pos: file.Package, Category: "TS-L16", Message: commentOnlyMessage,
			})
		}
	}
	if len(docs) > 1 {
		for _, doc := range docs {
			pass.Report(analysis.Diagnostic{
				Pos: doc.Pos(), Category: "TS-L15", Message: splitMessage,
			})
		}
	} else if count > 1 {
		pass.Report(analysis.Diagnostic{
			Pos: docs[0].Pos(), Category: "TS-L15", Message: longMessage,
		})
	}
	return nil, nil
}

func exempt(pass *analysis.Pass, file *ast.File) bool {
	name := pass.Fset.Position(file.Pos()).Filename
	return ast.IsGenerated(file) ||
		strings.HasSuffix(name, "_test.go") ||
		!strings.HasSuffix(name, ".go")
}

func blocks(group *ast.CommentGroup) int {
	if group == nil {
		return 0
	}
	var parser comment.Parser
	return len(parser.Parse(group.Text()).Content)
}
