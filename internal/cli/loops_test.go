package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// loopFinding matches a finding line from one of the loop rules.
var loopFinding = regexp.MustCompile(`^(\S+\.go):(\d+):\d+: (TS-S02|TS-S09|TS-V01): `)

// blockedFunctions names the functions in dir that stdout reports a loop
// finding in, sorted and without repeats.
func blockedFunctions(t *testing.T, dir string, got outcome) []string {
	t.Helper()
	found := map[string]bool{}
	for _, line := range strings.Split(got.stdout, "\n") {
		match := loopFinding.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		number, err := strconv.Atoi(match[2])
		require.NoError(t, err)
		name := functionAt(t, filepath.Join(dir, match[1]), number)
		require.NotEmpty(t, name)
		found[name] = true
	}
	return slices.Sorted(maps.Keys(found))
}

// functionAt returns the name of the function declared around line in file,
// qualified by its receiver type for a method.
func functionAt(t *testing.T, file string, line int) string {
	t.Helper()
	source, err := os.ReadFile(file)
	require.NoError(t, err)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, source, 0)
	require.NoError(t, err)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fset.Position(fn.Pos()).Line > line || fset.Position(fn.End()).Line < line {
			continue
		}
		if fn.Recv == nil {
			return fn.Name.Name
		}
		return strings.TrimPrefix(types.ExprString(fn.Recv.List[0].Type), "*") + "." + fn.Name.Name
	}
	return ""
}

// TestCheckBlocksEveryLoopThatNeverEndsOrStopsQuietly runs tiger over loops
// that look bounded to a reader and are not: counters the body moves away
// or resets, limits the body grows, proofs a shadow or a pointer defeats,
// caps that stop without a word, an iterator nobody can see into.
//
// Goal: each loop's function carries a loop finding, and the loops in the
// same module that end are left alone. WalkRange (ranging over a slice
// while appending ends, so it is a correctness bug for tests, not a loop
// rule) and ScanForever (a standard library stream over an endless source
// counts as an outside stream) are the recorded misses.
func TestCheckBlocksEveryLoopThatNeverEndsOrStopsQuietly(t *testing.T) {
	const dir = "testdata/fixtures/loopholes"
	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.ElementsMatch(t, []string{
		"AliasAppend", "ByteWraps", "ChunksParamMax", "ClosureGrow", "CounterReset",
		"CounterUndone", "CountVariant", "DrainRefill", "DrainSkip", "FieldNamedLikeCounter",
		"GotoLoop", "HelperGrow", "LabeledContinue", "LimitGrows", "MapGrow", "OrCounter",
		"Overflow", "ParsePackRaw", "RangeNaturals", "ShadowSlice", "Spinner.Spin",
		"CreateRaw", "ListRaw", "SpinFromConfigRaw",
		"SpinBreak", "SpinCapped", "SpinLimited", "StepsPastLimit", "Walk", "WalkCapped",
		"WrongCounter", "WrongWay", "naturals",
	}, blockedFunctions(t, dir, got))
}

// TestCheckPassesLoopsThatEndForReasonsTigerCanProve runs tiger over the
// correct loops that blocked before: git-server's sideband writer, varint
// encoders, a Myers diff walk, two cursors closing in, a queue field popped
// one item at a time, plus the compliant shape of each cap.
//
// Goal: no finding, exit 0.
func TestCheckPassesLoopsThatEndForReasonsTigerCanProve(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/loopsounds", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestCheckPassesTheRewriteOfEachBlockedLoop runs tiger over the compliant
// rewrite of the loops that TestCheckBlocksEveryLoopThatNeverEndsOrStopsQuietly
// blocks: the same shape with the hole closed.
//
// Goal: no finding, exit 0.
func TestCheckPassesTheRewriteOfEachBlockedLoop(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/loopfixes", "./...")
	assert.Empty(t, got.stdout)
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
}

// TestCheckPassesTigerItself runs tiger over its own module, which carries
// the capped worklists that replaced its uncapped ones.
//
// Goal: no finding, exit 0.
func TestCheckPassesTigerItself(t *testing.T) {
	got := run(t, "check", "-C", "../..", "./...")
	assert.Empty(t, got.stdout)
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
}

// TestCheckBlocksAnIteratorFromAModuleWithoutADotInItsPath ranges over an
// iterator written in the module itself, which is named myapp.
//
// Goal: a module path with no dot is not mistaken for the standard library,
// so the range is a TS-S02 finding.
func TestCheckBlocksAnIteratorFromAModuleWithoutADotInItsPath(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/loopdotless", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Contains(t, got.stdout,
		"loop.go:8:2: TS-S02: this loop ranges over an iterator tiger can't see into")
}
