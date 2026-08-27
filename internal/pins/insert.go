package pins

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/directive"
)

// Insertion is one additive text edit: Text spliced into the file's source
// at byte Offset, which always falls on a line boundary. Applying it never
// touches an existing byte — insertion is the only operation this package's
// write side offers.
type Insertion struct {
	Offset int
	Text   string
}

// Target is one insertion request: the node the directives bind to, in
// its parsed file, with the file's source bytes for offsets and
// indentation.
type Target struct {
	Fset       *token.FileSet
	File       *ast.File
	Src        []byte
	Node       ast.Node
	Directives []directive.Directive
}

// Insert returns the insertion that makes Collect bind the target's
// directives to its node: the same attachment rule Collect reads — a
// directive binds to the node starting on the line after its comment
// group ends — run backwards.
//
// The directive lines go at the start of the node's own line, in the
// order given, indented to the node's indentation. When a comment group
// ends on the line above the node (its doc group under Collect's rule)
// the lines extend that group; a group ending in prose first gets one
// bare // separator line, matching Go's //go: directive convention, while
// a group already ending in a directive takes the lines with no
// separator.
//
// A node outside the file, or an empty directive list, is a programming
// error, not a runtime condition.
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

// needsSeparator reports whether the node starting on line has a doc
// comment group ending in prose — the one case that takes a bare //
// between the prose and the pins.
func needsSeparator(fset *token.FileSet, file *ast.File, line int) bool {
	for _, group := range file.Comments {
		if fset.Position(group.End()).Line != line-1 {
			continue
		}
		last := group.List[len(group.List)-1]
		return !directive.Is(last.Text)
	}
	return false
}
