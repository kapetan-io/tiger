// Package comments enforces TS-L14: no prose comments below the package line
// of a production file. A //nolint that is not spelled exactly //nolint is
// honored by golangci-lint but is prose here.
package comments

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/config"
	"github.com/kapetan-io/tiger/internal/directive"
)

var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	built := &analysis.Analyzer{
		Name: "comments",
		Doc:  "TS-L14: no prose comments below the package line of a production file.",
		Run:  run,
	}
	built.Flags.Var(directoryList, "allow", "directories exempt from the comment ban, "+
		"each with a reason (comments.allow entries in tiger.yaml only)")
	return built
}

var directoryList = config.Directories{Name: "comments.allow"}

const message = "TS-L14: this comment is below the package line of a production file — " +
	"tiger expects the code, its tests and //tiger: directives to carry meaning here, " +
	"not prose — move what it says into a test, or delete it"

func run(pass *analysis.Pass) (any, error) {
	if len(config.Values(pass.Pkg.Path(), config.Key{Analyzer: "comments", Flag: "allow"})) > 0 {
		return nil, nil
	}
	for _, file := range pass.Files {
		if exempt(pass, file) {
			continue
		}
		for _, group := range file.Comments {
			if prose := firstProse(file, group); prose != nil {
				pass.Report(analysis.Diagnostic{
					Pos:      prose.Pos(),
					Category: "TS-L14",
					Message:  message,
				})
			}
		}
	}
	return nil, nil
}

func exempt(pass *analysis.Pass, file *ast.File) bool {
	return ast.IsGenerated(file) ||
		strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go")
}

func firstProse(file *ast.File, group *ast.CommentGroup) *ast.Comment {
	for _, comment := range group.List {
		class := directive.Classify(file, comment)
		if class == directive.Prose {
			return comment
		}
		if class != directive.Nolint || directive.Spelled(comment.Text) {
			continue
		}
		if comment.Pos() > file.Package {
			return comment
		}
	}
	return nil
}
