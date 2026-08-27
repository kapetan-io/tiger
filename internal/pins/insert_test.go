package pins_test

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/directive"
	"github.com/kapetan-io/tiger/internal/pins"
)

const unpinned = `package fixture

func Bare(r *int) {}

// Flush persists the buffered entries.
func Flush(r *int) {}

//tiger:batched provider offers no bulk endpoint
func Batched(r *int) {}

func Drain(pending []int) int {
	total := 0
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}
`

// parsed is one fixture source string's parse result.
type parsed struct {
	fset *token.FileSet
	file *ast.File
}

// parseSource parses one fixture source string with comments.
func parseSource(t *testing.T, src string) parsed {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, parser.ParseComments)
	require.NoError(t, err)
	return parsed{fset: fset, file: file}
}

// apply splices one insertion into src.
func apply(src string, insertion pins.Insertion) string {
	return src[:insertion.Offset] + insertion.Text + src[insertion.Offset:]
}

// tripped is one round trip's result: the re-parsed file and the pins
// collected from it.
type tripped struct {
	collected pins.Set
	file      *ast.File
}

// roundTrip applies insertion to src, asserts the result still parses and
// is gofmt-clean, and re-collects.
func roundTrip(t *testing.T, src string, insertion pins.Insertion) tripped {
	t.Helper()
	written := apply(src, insertion)
	formatted, err := format.Source([]byte(written))
	require.NoError(t, err)
	assert.Equal(t, written, string(formatted))
	reparsed := parseSource(t, written)
	return tripped{
		collected: pins.Collect(reparsed.fset, []*ast.File{reparsed.file}),
		file:      reparsed.file,
	}
}

// TestInsertWithNoDocComment covers the first placement case.
//
// Goal: pins on an undocumented declaration land directly above it, stack
// in the given order, and are collected back bound to that declaration.
func TestInsertWithNoDocComment(t *testing.T) {
	loaded := parseSource(t, unpinned)
	bare := find(loaded.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Bare"
	})
	require.NotNil(t, bare)

	insertion := pins.Insert(pins.Target{
		Fset: loaded.fset, File: loaded.file, Src: []byte(unpinned), Node: bare,
		Directives: []directive.Directive{
			{Verb: "effects", Args: "none"},
			{Verb: "frame", Args: "none"},
		},
	})
	assert.Equal(t, "//tiger:effects none\n//tiger:frame none\n", insertion.Text)

	trip := roundTrip(t, unpinned, insertion)
	rebound := find(trip.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Bare"
	})
	require.NotNil(t, rebound)
	require.Len(t, trip.collected.At(rebound, "effects"), 1)
	require.Len(t, trip.collected.At(rebound, "frame"), 1)
	assert.Equal(t, "none", trip.collected.At(rebound, "effects")[0].Directive.Args)
}

// TestInsertAfterProseDocComment covers the second placement case.
//
// Goal: pins under a prose doc comment are separated from it by one bare
// // line, extend the same comment group, and bind to the declaration.
func TestInsertAfterProseDocComment(t *testing.T) {
	loaded := parseSource(t, unpinned)
	flush := find(loaded.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Flush"
	})
	require.NotNil(t, flush)

	insertion := pins.Insert(pins.Target{
		Fset: loaded.fset, File: loaded.file, Src: []byte(unpinned), Node: flush,
		Directives: []directive.Directive{{Verb: "effects", Args: "mutate(r)"}},
	})
	assert.Equal(t, "//\n//tiger:effects mutate(r)\n", insertion.Text)

	trip := roundTrip(t, unpinned, insertion)
	rebound := find(trip.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Flush"
	})
	require.NotNil(t, rebound)
	require.Len(t, trip.collected.At(rebound, "effects"), 1)
	assert.Equal(t, "mutate(r)", trip.collected.At(rebound, "effects")[0].Directive.Args)
}

// TestInsertAfterDirectiveDocComment covers the third placement case.
//
// Goal: pins under a doc comment already ending in a directive append with
// no separator, and every original byte survives.
func TestInsertAfterDirectiveDocComment(t *testing.T) {
	loaded := parseSource(t, unpinned)
	batched := find(loaded.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Batched"
	})
	require.NotNil(t, batched)

	insertion := pins.Insert(pins.Target{
		Fset: loaded.fset, File: loaded.file, Src: []byte(unpinned), Node: batched,
		Directives: []directive.Directive{{Verb: "effects", Args: "none"}},
	})
	assert.Equal(t, "//tiger:effects none\n", insertion.Text)

	written := apply(unpinned, insertion)
	assert.Contains(t, written,
		"//tiger:batched provider offers no bulk endpoint\n//tiger:effects none\nfunc Batched")

	trip := roundTrip(t, unpinned, insertion)
	rebound := find(trip.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Batched"
	})
	require.NotNil(t, rebound)
	require.Len(t, trip.collected.At(rebound, "effects"), 1)
}

// TestInsertAboveNestedLoop covers loop placement.
//
// Goal: a variant pin on a loop nested inside a body is indented to the
// loop's own indentation and collected back bound to the loop statement.
func TestInsertAboveNestedLoop(t *testing.T) {
	loaded := parseSource(t, unpinned)
	loop := find(loaded.file, func(node ast.Node) bool {
		_, ok := node.(*ast.ForStmt)
		return ok
	})
	require.NotNil(t, loop)

	insertion := pins.Insert(pins.Target{
		Fset: loaded.fset, File: loaded.file, Src: []byte(unpinned), Node: loop,
		Directives: []directive.Directive{{Verb: "variant", Args: "len(pending)"}},
	})
	assert.Equal(t, "\t//tiger:variant len(pending)\n", insertion.Text)

	trip := roundTrip(t, unpinned, insertion)
	rebound := find(trip.file, func(node ast.Node) bool {
		_, ok := node.(*ast.ForStmt)
		return ok
	})
	require.NotNil(t, rebound)
	require.Len(t, trip.collected.At(rebound, "variant"), 1)
	assert.Equal(t, "len(pending)", trip.collected.At(rebound, "variant")[0].Directive.Args)

	drain := find(trip.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Drain"
	})
	require.NotNil(t, drain)
	assert.Empty(t, trip.collected.At(drain, "variant"))
}

// TestInsertTextIsFormatOutput covers state invariant 1 at the writer.
//
// Goal: every directive line in the insertion text is byte-exact
// directive.Format output plus indentation and newline — nothing else.
func TestInsertTextIsFormatOutput(t *testing.T) {
	loaded := parseSource(t, unpinned)
	bare := find(loaded.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Bare"
	})
	require.NotNil(t, bare)

	insertion := pins.Insert(pins.Target{
		Fset: loaded.fset, File: loaded.file, Src: []byte(unpinned), Node: bare,
		Directives: []directive.Directive{{Verb: "effects", Args: "alloc, io(disk)"}},
	})
	for _, line := range strings.Split(strings.TrimSuffix(insertion.Text, "\n"), "\n") {
		trimmed := strings.TrimLeft(line, "\t ")
		if trimmed == "//" {
			continue
		}
		parsed, err := directive.Parse(trimmed)
		require.NoError(t, err)
		assert.Equal(t, trimmed, directive.Format(parsed))
	}
}
