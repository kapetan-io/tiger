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

// pinnedConsumeGolden is the showfacts fixture's facts.go after
// `tiger pin Consume`: the variant from the --show-facts golden output,
// indented above Consume's loop, and every original byte intact.
const pinnedConsumeGolden = `// Package showfacts holds unpinned loops whose synthesized variants
// print under --show-facts in freeze-ready pin syntax, and nothing else.
package showfacts

// Recorder totals entries.
type Recorder struct{}

// Consume totals pending and empties it.
func (r *Recorder) Consume(pending []int) int {
	total := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
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
`

// TestPinWritesFactsForNamedTarget covers the core acceptance criterion.
//
// Goal: pin Consume writes exactly the directive the fixture's
// --show-facts golden output prints for Consume, the resulting file bytes
// match the committed expectation, the written line prints with its file
// and final line number, and the run exits 0.
func TestPinWritesFactsForNamedTarget(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	got := run(t, "pin", "-C", dir, "Consume", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "facts.go:11: //tiger:variant len(pending)\n", got.stdout)
	assert.Equal(t, pinnedConsumeGolden, readFile(t, filepath.Join(dir, "facts.go")))
}

// TestPinQualifiedMethodName covers method-name resolution.
//
// Goal: the Go doc forms (*Recorder).Consume and Recorder.Consume resolve to
// the same method a bare Consume does.
func TestPinQualifiedMethodName(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	got := run(t, "pin", "-C", dir, "(*Recorder).Consume", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Equal(t, pinnedConsumeGolden, readFile(t, filepath.Join(dir, "facts.go")))

	again := copyFixture(t, "showfacts")
	viaValue := run(t, "pin", "-C", again, "Recorder.Consume", "./...")
	assert.Equal(t, cli.ExitClean, viaValue.code)
	assert.Equal(t, pinnedConsumeGolden, readFile(t, filepath.Join(again, "facts.go")))
}

// TestPinSecondRunWritesNothing covers invariant 4.
//
// Goal: a second run over the same targets writes nothing, prints nothing,
// and exits 0 — a satisfied pin is a satisfied target, not a duplicate.
func TestPinSecondRunWritesNothing(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	first := run(t, "pin", "-C", dir, "Consume", "./...")
	require.Equal(t, cli.ExitClean, first.code)
	pinned := readFile(t, filepath.Join(dir, "facts.go"))

	second := run(t, "pin", "-C", dir, "Consume", "./...")
	assert.Equal(t, cli.ExitClean, second.code)
	assert.Empty(t, second.stdout)
	assert.Empty(t, second.stderr)
	assert.Equal(t, pinned, readFile(t, filepath.Join(dir, "facts.go")))
}

// TestPinnedTreeChecksClean covers invariant 6 end to end.
//
// Goal: after pin writes every target's facts, check over the same tree
// exits clean with no findings, and --show-facts has nothing left to print.
func TestPinnedTreeChecksClean(t *testing.T) {
	dir := copyFixture(t, "showfacts")
	got := run(t, "pin", "-C", dir, "Consume", "Drain", "./...")
	require.Equal(t, cli.ExitClean, got.code)
	require.Empty(t, got.stderr)

	checked := run(t, "check", "-C", dir, "--show-facts", "./...")
	assert.Equal(t, cli.ExitClean, checked.code)
	assert.Empty(t, checked.stdout)
	assert.Empty(t, checked.stderr)
}

// TestPinUnexportedTargetWritesVariant covers the unexported policy.
//
// Goal: an unexported target takes variant pins for the loops in its body,
// exactly as an exported one does.
func TestPinUnexportedTargetWritesVariant(t *testing.T) {
	dir := copyFixture(t, "mixed")
	got := run(t, "pin", "-C", dir, "count", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "mixed.go:33: //tiger:variant len(pending)\n", got.stdout)

	written := readFile(t, filepath.Join(dir, "mixed.go"))
	assert.Contains(t, written, "\t//tiger:variant len(pending)\n\tfor len(pending) > 0 {")
}

// TestPinDisagreeingPinRefusedWithPartialProgress covers the refusal and
// partial-progress contracts together.
//
// Goal: a target whose existing pin disagrees with the computed fact is
// refused — the pinned line and the finding print, with the two exits
// named — while the other named target still writes, and the run exits 1.
func TestPinDisagreeingPinRefusedWithPartialProgress(t *testing.T) {
	dir := copyFixture(t, "mixed")
	before := readFile(t, filepath.Join(dir, "mixed.go"))
	got := run(t, "pin", "-C", dir, "Good", "Bad", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "refused Bad")
	assert.Contains(t, got.stdout, "pinned: //tiger:variant len(names)")
	assert.Contains(t, got.stdout, "change the code, or edit the pin by hand")

	written := readFile(t, filepath.Join(dir, "mixed.go"))
	assert.Contains(t, written, "\t//tiger:variant len(pending)\n\tfor len(pending) > 0 {")
	assert.Contains(t, written, "//tiger:variant len(names)\n\tfor len(pending) > 0 {")
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
	got := run(t, "pin", "-C", dir, "Consumee", "./...")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Contains(t, got.stderr, "Consumee")
	assert.Contains(t, got.stderr, "Consume")
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
	got := run(t, "pin", "-C", dir, "--dry-run", "Consume", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Equal(t, "facts.go:11: //tiger:variant len(pending)\n", got.stdout)
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
