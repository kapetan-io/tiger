package placement

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/directive"
)

type Insertion struct {
	Offset int
	Text   string
}

type Target struct {
	Fset       *token.FileSet
	File       *ast.File
	Src        []byte
	Node       ast.Node
	Directives []directive.Directive
}

func Insert(target Target) Insertion {
	assert.Ok(len(target.Directives) > 0, "Insert needs at least one directive")
	assert.Ok(
		target.File.FileStart <= target.Node.Pos(), "Insert target node starts before the file",
	)
	assert.Ok(target.Node.End() <= target.File.FileEnd, "Insert target node ends after the file")
	position := target.Fset.Position(target.Node.Pos())
	lines := target.Fset.File(target.Node.Pos())
	offset := lines.Offset(lines.LineStart(position.Line))
	indent := string(target.Src[offset : offset+position.Column-1])
	assert.Ok(strings.TrimLeft(indent, "\t ") == "", "Insert target does not start its line")

	text := strings.Builder{}
	if needsSeparator(target.Fset, target.File, position.Line) {
		text.WriteString(indent + "//\n")
	}
	for _, d := range target.Directives {
		text.WriteString(indent + directive.Format(d) + "\n")
	}
	return Insertion{Offset: offset, Text: text.String()}
}

func needsSeparator(fset *token.FileSet, file *ast.File, line int) bool {
	for _, group := range file.Comments {
		if fset.Position(group.End()).Line != line-1 {
			continue
		}
		last := group.List[len(group.List)-1]
		return directive.Classify(file, last) != directive.Tiger
	}
	return false
}
