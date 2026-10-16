package ssalib_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/ssalib"
)

const fixtureSource = `package fixture

type Ring struct {
	log []byte
}

func (r *Ring) Append(entry byte) {
	r.log = append(r.log, entry)
}

func Spawn(r *Ring) {
	go r.Append(1)
}

func Relay(r *Ring) {
	r.Append(2)
}

func Even(n int) bool {
	if n == 0 {
		return true
	}
	return Odd(n - 1)
}

func Odd(n int) bool {
	if n == 0 {
		return false
	}
	return Even(n - 1)
}

func Solo() {}
`

// build compiles the fixture to SSA in memory.
func build(t *testing.T) *ssa.Package {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", fixtureSource, 0)
	require.NoError(t, err)
	shell := types.NewPackage("fixture", "fixture")
	config := &types.Config{Importer: importer.Default()}
	files := []*ast.File{file}
	built, _, err := ssautil.BuildPackage(config, fset, shell, files, ssa.BuilderMode(0))
	require.NoError(t, err)
	return built
}

// fn locates one function or method by name in the built package.
func fn(t *testing.T, pkg *ssa.Package, name string) *ssa.Function {
	t.Helper()
	if member, ok := pkg.Members[name].(*ssa.Function); ok {
		return member
	}
	ring := pkg.Type("Ring").Type()
	pointer := types.NewPointer(ring)
	method := pkg.Prog.LookupMethod(pointer, pkg.Pkg, name)
	require.NotNil(t, method)
	return method
}

// TestStaticCallsResolvesDirectAndSpawned covers static call enumeration.
//
// Goal: direct calls and go statements resolve to their static callee with
// position and argument values, and the spawned flag distinguishes them.
func TestStaticCallsResolvesDirectAndSpawned(t *testing.T) {
	pkg := build(t)

	spawned := ssalib.StaticCalls(fn(t, pkg, "Spawn"))
	require.Len(t, spawned, 1)
	assert.Equal(t, "Append", spawned[0].Callee.Name())
	assert.True(t, spawned[0].Spawned)
	assert.True(t, spawned[0].Pos.IsValid())

	relayed := ssalib.StaticCalls(fn(t, pkg, "Relay"))
	require.Len(t, relayed, 1)
	assert.Equal(t, "Append", relayed[0].Callee.Name())
	assert.False(t, relayed[0].Spawned)
	assert.Len(t, relayed[0].Args, 2)
}

// TestComponentsFindsCyclesInReverseTopologicalOrder covers the shared SCC
// walk norecursion rides on.
//
// Goal: mutual recursion lands in one component, singletons stay single,
// and callees' components complete before their callers'.
func TestComponentsFindsCyclesInReverseTopologicalOrder(t *testing.T) {
	pkg := build(t)
	appendFn := fn(t, pkg, "Append")
	relay := fn(t, pkg, "Relay")
	even := fn(t, pkg, "Even")
	odd := fn(t, pkg, "Odd")
	nodes := []*ssa.Function{relay, appendFn, even, odd, fn(t, pkg, "Solo")}

	adjacency := func(member *ssa.Function) []*ssa.Function {
		called := []*ssa.Function{}
		for _, call := range ssalib.StaticCalls(member) {
			called = append(called, call.Callee)
		}
		return called
	}
	components := ssalib.Components(nodes, adjacency)
	require.Len(t, components, 4)

	position := map[*ssa.Function]int{}
	for at, component := range components {
		for _, member := range component {
			position[member] = at
		}
	}
	assert.Equal(t, position[even], position[odd])
	assert.Less(t, position[appendFn], position[relay])

	again := ssalib.Components(nodes, adjacency)
	assert.Equal(t, components, again)
}
