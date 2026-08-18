package pins_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/pins"
)

const pinned = `package fixture

// Flush persists the buffered entries.
//
//tiger:effects mutate(r.log), io(disk)
//tiger:frame r.log
func Flush(r *int) {}

//tiger:batched provider offers no bulk endpoint
func Notify() {}

//tiger:effects io(dsk)
func Malformed() {}

func Drain(pending []int) int {
	total := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}
`

// fixture is the parsed pin fixture: its collected pins and its file.
type fixture struct {
	collected pins.Set
	file      *ast.File
}

// parse loads the fixture source and collects its pins.
func parse(t *testing.T) fixture {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", pinned, parser.ParseComments)
	require.NoError(t, err)
	return fixture{collected: pins.Collect(fset, []*ast.File{file}), file: file}
}

// find returns the first declaration or statement satisfying match.
func find(file *ast.File, match func(ast.Node) bool) ast.Node {
	var found ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if found == nil && node != nil && match(node) {
			found = node
		}
		return found == nil
	})
	return found
}

// TestCollectBindsPinsToDeclarations covers pin attachment.
//
// Goal: a pin directive in a declaration's doc group binds to that
// declaration, stacked pins all bind, and each pin carries its parsed
// directive and its comment position for findings.
func TestCollectBindsPinsToDeclarations(t *testing.T) {
	loaded := parse(t)
	flush := find(loaded.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Flush"
	})
	require.NotNil(t, flush)

	effects := loaded.collected.At(flush, "effects")
	require.Len(t, effects, 1)
	assert.Equal(t, "mutate(r.log), io(disk)", effects[0].Directive.Args)
	assert.True(t, effects[0].Pos.IsValid())

	frames := loaded.collected.At(flush, "frame")
	require.Len(t, frames, 1)
	assert.Equal(t, "r.log", frames[0].Directive.Args)

	assert.Empty(t, loaded.collected.At(flush, "variant"))
}

// TestCollectBindsLoopPins covers statement attachment.
//
// Goal: a variant pin on the line above a loop binds to the loop statement,
// not to the enclosing function.
func TestCollectBindsLoopPins(t *testing.T) {
	loaded := parse(t)
	loop := find(loaded.file, func(node ast.Node) bool {
		_, ok := node.(*ast.ForStmt)
		return ok
	})
	require.NotNil(t, loop)

	variants := loaded.collected.At(loop, "variant")
	require.Len(t, variants, 1)
	assert.Equal(t, "len(pending)", variants[0].Directive.Args)

	drain := find(loaded.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Drain"
	})
	require.NotNil(t, drain)
	assert.Empty(t, loaded.collected.At(drain, "variant"))
}

// TestCollectSkipsNonPinsAndMalformed covers the collection boundary.
//
// Goal: escape directives and pins that fail their verb's grammar are not
// collected — the directives analyzer owns their findings — so consumers
// only ever see well-formed pins.
func TestCollectSkipsNonPinsAndMalformed(t *testing.T) {
	loaded := parse(t)
	notify := find(loaded.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Notify"
	})
	require.NotNil(t, notify)
	assert.Empty(t, loaded.collected.At(notify, "batched"))

	malformed := find(loaded.file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		return ok && decl.Name.Name == "Malformed"
	})
	require.NotNil(t, malformed)
	assert.Empty(t, loaded.collected.At(malformed, "effects"))
}
