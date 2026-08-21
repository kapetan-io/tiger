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
	"github.com/kapetan-io/tiger/internal/directive"
)

const fixtureSource = `package fixture

type Ring struct {
	log   []byte
	next  *Ring
	cache map[string]int
}

func (r *Ring) Append(entry byte) {
	r.log = append(r.log, entry)
}

func (r *Ring) Touch(k string) {
	r.cache[k] = 1
}

func (r *Ring) Deep() {
	r.next.log = nil
}

func Spawn(r *Ring) {
	go r.Append(1)
}

func Relay(r *Ring) {
	r.Append(2)
}

func Local() {
	scratch := Ring{}
	scratch.log = nil
	_ = scratch
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

func Wait(c chan int) int {
	return <-c
}

func Boom() {
	panic("boom")
}

func Fresh() map[string]int {
	return map[string]int{}
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

// TestWritesFindsParameterRootedPaths covers the write-path fragment shared
// by effects (mutate) and frames.
//
// Goal: stores and map updates whose address chases through field and index
// selections to a parameter come back as index-rooted locations — including
// paths through pointer loads — while writes to locals are dropped.
func TestWritesFindsParameterRootedPaths(t *testing.T) {
	pkg := build(t)

	appendWrites := ssalib.Writes(fn(t, pkg, "Append"))
	require.Len(t, appendWrites, 1)
	assert.Equal(t, ssalib.Location{Param: 0, Path: ".log"}, appendWrites[0].Loc)
	assert.True(t, appendWrites[0].Pos.IsValid())

	touchWrites := ssalib.Writes(fn(t, pkg, "Touch"))
	require.Len(t, touchWrites, 1)
	assert.Equal(t, ssalib.Location{Param: 0, Path: ".cache"}, touchWrites[0].Loc)

	deepWrites := ssalib.Writes(fn(t, pkg, "Deep"))
	require.Len(t, deepWrites, 1)
	assert.Equal(t, ssalib.Location{Param: 0, Path: ".next.log"}, deepWrites[0].Loc)

	assert.Empty(t, ssalib.Writes(fn(t, pkg, "Local")))
	assert.Empty(t, ssalib.Writes(fn(t, pkg, "Solo")))
}

// TestRenderAndRebase covers the name seam between index-rooted locations
// and pin syntax, and the modular rebase at call sites.
//
// Goal: a location renders through the function's own parameter names, and
// a callee location rebases to the caller's parameter when the caller
// passes its parameter straight through.
func TestRenderAndRebase(t *testing.T) {
	pkg := build(t)
	appendFn := fn(t, pkg, "Append")

	rendered, ok := ssalib.RenderLocation(appendFn, ssalib.Location{Param: 0, Path: ".log"})
	require.True(t, ok)
	assert.Equal(t, "r.log", rendered)

	_, ok = ssalib.RenderLocation(appendFn, ssalib.Location{Param: 9, Path: ""})
	assert.False(t, ok)

	relay := fn(t, pkg, "Relay")
	calls := ssalib.StaticCalls(relay)
	require.Len(t, calls, 1)
	rebased, ok := ssalib.RebaseArg(calls[0].Args, ssalib.Location{Param: 0, Path: ".log"})
	require.True(t, ok)
	assert.Equal(t, ssalib.Location{Param: 0, Path: ".log"}, rebased)
	rendered, ok = ssalib.RenderLocation(relay, rebased)
	require.True(t, ok)
	assert.Equal(t, "r.log", rendered)
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
// walk norecursion and the effects closure ride on.
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

// TestInstructionEffects covers the own-instruction effect sources.
//
// Goal: go statements contribute spawn, allocation instructions and the
// append builtin contribute alloc, channel receives contribute block, and
// panic contributes panic — each with an origin position for provenance.
func TestInstructionEffects(t *testing.T) {
	pkg := build(t)
	for _, test := range []struct {
		name string
		fn   string
		want string
		atom string
	}{
		{name: "GoContributesSpawn", fn: "Spawn", want: "spawn", atom: "spawn"},
		{name: "AppendContributesAlloc", fn: "Append", want: "alloc", atom: "alloc"},
		{name: "ReceiveContributesBlock", fn: "Wait", want: "block", atom: "block"},
		{name: "PanicContributesPanic", fn: "Boom", want: "panic", atom: "panic"},
		{name: "MakeMapContributesAlloc", fn: "Fresh", want: "alloc", atom: "alloc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			found := ssalib.InstructionEffects(fn(t, pkg, test.fn))
			assert.Equal(t, test.want, directive.FormatEffects(found.Set))
			require.Contains(t, found.Origins, test.atom)
			assert.True(t, found.Origins[test.atom].Pos.IsValid())
			assert.NotEmpty(t, found.Origins[test.atom].Detail)
		})
	}

	pure := ssalib.InstructionEffects(fn(t, pkg, "Solo"))
	assert.True(t, pure.Set.Empty())
	assert.Empty(t, pure.Origins)
}
