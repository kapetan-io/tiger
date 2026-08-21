// Package attach resolves which tiger directive, if any, is attached to a
// node.
//
// This is the shared attachment contract for directive consumers: a
// directive governs a node when it sits in the comment group ending on the
// line immediately above the node, or trails on the node's own first line.
// Only a well-formed directive attaches — an escape with no reason is a
// blocking TS-L09 finding, never a working waiver — and directive
// validation itself stays solely in the directives analyzer.
package attach

import (
	"go/ast"
	"go/token"

	"github.com/kapetan-io/tiger/internal/directive"
)

// Directive returns the well-formed //tiger:<verb> directive attached to
// node, if one is present in file's comments.
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
			if !directive.Is(comment.Text) {
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
