package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// copyFixture copies one fixture module into a fresh temp dir, because pin
// mutates the tree it runs over — the committed fixtures stay read-only.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(filepath.Join("testdata", "fixtures", name))))
	return dir
}

// readFile returns one file's bytes from a fixture copy.
func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// pinnedAppendGolden is the showfacts fixture's facts.go after
// `tiger pin Append`: the two directives from the --show-facts golden
// output, appended to Append's doc group behind one bare // separator, and
// every original byte intact.
const pinnedAppendGolden = `// Package showfacts holds unpinned exported functions whose computed
// facts — effect sets, frames, a synthesized variant — print under
// --show-facts in freeze-ready pin syntax, and nothing else.
package showfacts

import "os"

// Recorder accumulates entries.
type Recorder struct {
	log []byte
}

// Append records one entry.
//
//tiger:effects alloc, mutate(r.log)
//tiger:frame r.log
func (r *Recorder) Append(entry byte) {
	r.log = append(r.log, entry)
}

// Drain consumes pending and totals it.
func Drain(pending []int) int {
	total := 0
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}

// Home reads the configured home directory.
func Home() string {
	return os.Getenv("TIGER_HOME")
}
`

// TestPinWritesFactsForNamedTarget covers the core acceptance criterion.
//
// Goal: pin Append writes exactly the directives the fixture's
// --show-facts golden output prints for Append, the resulting file bytes
// match the committed expectation, each written line prints with its file
// and final line number, and the run exits 0.
func TestPinWritesFactsForNamedTarget(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	got := run(t, "pin", "-C", dir, "Append", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "facts.go:15: //tiger:effects alloc, mutate(r.log)\n"+
		"facts.go:16: //tiger:frame r.log\n", got.stdout)
	assert.Equal(t, pinnedAppendGolden, readFile(t, filepath.Join(dir, "facts.go")))
}

// TestPinQualifiedMethodName covers method-name resolution.
//
// Goal: the Go doc forms (*Recorder).Append and Recorder.Append resolve to
// the same method a bare Append does.
func TestPinQualifiedMethodName(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	got := run(t, "pin", "-C", dir, "(*Recorder).Append", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Equal(t, pinnedAppendGolden, readFile(t, filepath.Join(dir, "facts.go")))

	again := copyFixture(t, "showfacts")
	viaValue := run(t, "pin", "-C", again, "Recorder.Append", "./...")
	assert.Equal(t, cli.ExitClean, viaValue.code)
	assert.Equal(t, pinnedAppendGolden, readFile(t, filepath.Join(again, "facts.go")))
}

// TestPinSecondRunWritesNothing covers invariant 4.
//
// Goal: a second run over the same targets writes nothing, prints nothing,
// and exits 0 — a satisfied pin is a satisfied target, not a duplicate.
func TestPinSecondRunWritesNothing(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	first := run(t, "pin", "-C", dir, "Append", "./...")
	require.Equal(t, cli.ExitClean, first.code)
	pinned := readFile(t, filepath.Join(dir, "facts.go"))

	second := run(t, "pin", "-C", dir, "Append", "./...")
	assert.Equal(t, cli.ExitClean, second.code)
	assert.Empty(t, second.stdout)
	assert.Empty(t, second.stderr)
	assert.Equal(t, pinned, readFile(t, filepath.Join(dir, "facts.go")))
}

// TestPinnedTreeChecksClean covers invariant 6 end to end.
//
// Goal: after pin writes every exported target's facts — including the
// variant above Drain's loop — check over the same tree exits clean with
// no findings, and --show-facts has nothing left to print.
func TestPinnedTreeChecksClean(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	got := run(t, "pin", "-C", dir, "Append", "Drain", "Home", "./...")
	require.Equal(t, cli.ExitClean, got.code)
	require.Empty(t, got.stderr)

	checked := run(t, "check", "-C", dir, "--show-facts", "./...")
	assert.Equal(t, cli.ExitClean, checked.code)
	assert.Empty(t, checked.stdout)
	assert.Empty(t, checked.stderr)
}

// TestPinUnexportedTargetWritesVariantOnly covers the unexported policy.
//
// Goal: an unexported target takes variant pins for the loops in its body,
// no effects or frame pin, and one informational line names the reason
// without affecting the exit code.
func TestPinUnexportedTargetWritesVariantOnly(t *testing.T) {
	dir := copyFixture(t, "mixed")
	got := run(t, "pin", "-C", dir, "count", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout, "count: unexported, no effects or frame pin")
	assert.Contains(t, got.stdout, "//tiger:variant len(pending)")

	written := readFile(t, filepath.Join(dir, "mixed.go"))
	assert.Contains(t, written, "\t//tiger:variant len(pending)\n\tfor len(pending) > 0 {")
	assert.NotContains(t, written, "//tiger:frame")
}

// TestPinDisagreeingPinRefusedWithPartialProgress covers the refusal and
// partial-progress contracts together.
//
// Goal: a target whose existing pin disagrees with the computed fact is
// refused — the pinned line and the computed effects print, with the two
// exits named — while the other named target still writes, and the run
// exits 1.
func TestPinDisagreeingPinRefusedWithPartialProgress(t *testing.T) {
	dir := copyFixture(t, "mixed")
	before := readFile(t, filepath.Join(dir, "mixed.go"))
	got := run(t, "pin", "-C", dir, "Good", "Bad", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "refused Bad")
	assert.Contains(t, got.stdout, "//tiger:effects none")
	assert.Contains(t, got.stdout, "io(env)")
	assert.Contains(t, got.stdout, "change the code, or edit the pin by hand")

	written := readFile(t, filepath.Join(dir, "mixed.go"))
	assert.Contains(t, written, "//tiger:effects io(env)\n//tiger:frame none\nfunc Good() string {")
	assert.Contains(t, written, "// Bad carries a pin that disagrees with its computed effects.\n"+
		"//\n//tiger:effects none\nfunc Bad() string {")
	assert.NotEqual(t, before, written)
}

// TestPinTargetWithBlockingFindingRefused covers behavioral constraint 8.
//
// Goal: a target carrying a blocking finding is refused with the finding
// printed, nothing is written for it, and the run exits 1.
func TestPinTargetWithBlockingFindingRefused(t *testing.T) {
	dir := copyFixture(t, "mixed")
	before := readFile(t, filepath.Join(dir, "mixed.go"))
	got := run(t, "pin", "-C", dir, "Panics", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "refused Panics")
	assert.Contains(t, got.stdout, "TS-S18")
	assert.Equal(t, before, readFile(t, filepath.Join(dir, "mixed.go")))
}

// TestPinAmbiguousNameWritesNothing covers the ambiguity contract.
//
// Goal: a name resolving to more than one function aborts the whole run —
// every match prints in qualified form, nothing writes anywhere, and the
// run exits 2.
func TestPinAmbiguousNameWritesNothing(t *testing.T) {
	dir := copyFixture(t, "ambiguous")
	alphaBefore := readFile(t, filepath.Join(dir, "alpha/alpha.go"))
	got := run(t, "pin", "-C", dir, "Flush", "./...")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "fixture.example/ambiguous/alpha.Flush")
	assert.Contains(t, got.stderr, "fixture.example/ambiguous/beta.Flush")
	assert.Equal(t, alphaBefore, readFile(t, filepath.Join(dir, "alpha/alpha.go")))
}

// TestPinUnknownNameExitsTwo covers the no-match contract.
//
// Goal: a name matching nothing is an operational error naming the closest
// declared function to retry with.
func TestPinUnknownNameExitsTwo(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	got := run(t, "pin", "-C", dir, "Appendd", "./...")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Contains(t, got.stderr, "Appendd")
	assert.Contains(t, got.stderr, "Append")
}

// TestPinNoNamesPrintsUsage covers the empty-invocation contract.
//
// Goal: pin with no names prints usage and exits 2.
func TestPinNoNamesPrintsUsage(t *testing.T) {
	got := run(t, "pin")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Contains(t, got.stderr, "usage:")
}

// TestPinDryRunLeavesTreeByteIdentical covers the --dry-run contract.
//
// Goal: --dry-run prints exactly what a real run would write, with its
// insertion points, and changes nothing on disk.
func TestPinDryRunLeavesTreeByteIdentical(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	before := readFile(t, filepath.Join(dir, "facts.go"))
	got := run(t, "pin", "-C", dir, "--dry-run", "Append", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Equal(t, "facts.go:15: //tiger:effects alloc, mutate(r.log)\n"+
		"facts.go:16: //tiger:frame r.log\n", got.stdout)
	assert.Equal(t, before, readFile(t, filepath.Join(dir, "facts.go")))
}

// TestPinGeneratedFileTargetRefused covers the generated-file policy.
//
// Goal: a target inside a machine-generated file is refused with a printed
// reason, matching the driver's ast.IsGenerated policy, and nothing writes.
func TestPinGeneratedFileTargetRefused(t *testing.T) {
	dir := copyFixture(t, "genpin")
	before := readFile(t, filepath.Join(dir, "machine.go"))
	got := run(t, "pin", "-C", dir, "Made", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "refused Made")
	assert.Contains(t, got.stdout, "generated file")
	assert.Equal(t, before, readFile(t, filepath.Join(dir, "machine.go")))
}
