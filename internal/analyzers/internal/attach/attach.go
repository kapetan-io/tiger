// Package attach resolves which tiger directive, if any, is attached to a
// node.
package attach

import (
	"go/ast"
	"go/token"

	"github.com/kapetan-io/tiger/internal/directive"
)

func Directive(
	fset *token.FileSet,
	file *ast.File,
	node ast.Node,
	verb string,
) (directive.Directive, bool) {
	nodeLine := fset.Position(node.Pos()).Line
	for _, group := range file.Comments {
		preceding := fset.Position(group.End()).Line == nodeLine-1
		trailing := fset.Position(group.Pos()).Line == nodeLine && group.Pos() > node.Pos()
		if !preceding && !trailing {
			continue
		}
		for _, comment := range group.List {
			if directive.Classify(file, comment) != directive.Tiger {
				continue
			}
			parsed, err := directive.Parse(comment.Text)
			if err == nil && parsed.Verb == verb {
				return parsed, true
			}
		}
	}
	return directive.Directive{Verb: "", Args: ""}, false
}
