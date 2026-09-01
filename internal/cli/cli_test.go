package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
	"github.com/kapetan-io/tiger/internal/directive"
)

// outcome is one CLI invocation's observable result.
type outcome struct {
	code   int
	stdout string
	stderr string
}

// run invokes the CLI surface once.
func run(t *testing.T, args ...string) outcome {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := cli.Run(args, cli.Streams{Stdout: stdout, Stderr: stderr})
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// TestCheckCleanTreeIsSilent covers the exit-0 contract.
//
// Goal: a conforming package produces no output and exit code 0.
func TestCheckCleanTreeIsSilent(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/clean", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// factsGolden is the showfacts fixture's complete --show-facts output:
// computed effect sets, frames, and a synthesized variant, every fact in
// freeze-ready pin syntax.
const factsGolden = "facts.go:14:1: TS-F01: Append's effects are alloc, mutate(r.log) — " +
	"nothing to fix; to make tiger fail the build if they change, add " +
	"//tiger:effects alloc, mutate(r.log)\n" +
	"facts.go:14:1: TS-F07: Append writes r.log through its receiver or parameters — " +
	"nothing to fix; to make tiger fail the build if that changes, add //tiger:frame r.log\n" +
	"facts.go:19:1: TS-F01: Drain has no effects — nothing to fix; to make tiger fail the " +
	"build if that changes, add //tiger:effects none\n" +
	"facts.go:19:1: TS-F07: Drain writes nothing through its receiver or parameters — " +
	"nothing to fix; to make tiger fail the build if that changes, add //tiger:frame none\n" +
	"facts.go:21:2: TS-V01: this loop in Drain ends because len(pending) shrinks on every pass — " +
	"nothing to fix; to make tiger fail the build if that changes, add " +
	"//tiger:variant len(pending)\n" +
	"facts.go:29:1: TS-F01: Home's effects are io(env) — nothing to fix; to make tiger " +
	"fail the build if they change, add //tiger:effects io(env)\n" +
	"facts.go:29:1: TS-F07: Home writes nothing through its receiver or parameters — " +
	"nothing to fix; to make tiger fail the build if that changes, add //tiger:frame none\n"

// TestCheckShowFactsPrintsPinSyntax covers the reported severity level's
// activation and invariant 1 at the CLI surface.
//
// Goal: --show-facts prints every computed fact — effect sets, frames,
// synthesized variants — as position-prefixed lines whose pin text is
// byte-exact directive.Format output, the run still exits 0, and no
// summary line appears because reported findings are never counted.
func TestCheckShowFactsPrintsPinSyntax(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/showfacts", "--show-facts", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, factsGolden, got.stdout)
}

// TestCheckShowFactsOutputRoundTrips covers invariant 1 end to end.
//
// Goal: every printed fact's //tiger: text parses back through the grammar
// to a directive that formats byte-identically, so ENG-151 can freeze any
// printed fact into a pin by pasting it.
func TestCheckShowFactsOutputRoundTrips(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/showfacts", "--show-facts", "./...")
	require.Equal(t, cli.ExitClean, got.code)
	lines := strings.Split(strings.TrimSuffix(got.stdout, "\n"), "\n")
	require.NotEmpty(t, lines)
	for _, line := range lines {
		at := strings.Index(line, "//tiger:")
		require.GreaterOrEqual(t, at, 0)
		pin := line[at:]
		parsed, err := directive.Parse(pin)
		require.NoError(t, err)
		assert.Equal(t, pin, directive.Format(parsed))
	}
}

// TestCheckWithoutShowFactsIsByteIdenticalToWaveOne covers the flag-absent
// contract.
//
// Goal: a tree whose only findings are computed facts prints nothing and
// exits 0 without the flag — byte-identical to a wave-1 run over the same
// tree, with no summary line, because reported findings are never counted.
func TestCheckWithoutShowFactsIsByteIdenticalToWaveOne(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/showfacts", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestCheckShowFactsIsDeterministic covers constraint 5 with the facts
// channel on.
//
// Goal: two --show-facts runs over the same tree are byte-identical.
func TestCheckShowFactsIsDeterministic(t *testing.T) {
	first := run(t, "check", "-C", "testdata/fixtures/showfacts", "--show-facts", "./...")
	second := run(t, "check", "-C", "testdata/fixtures/showfacts", "--show-facts", "./...")
	assert.Equal(t, first, second)
}

// TestCheckFactsPropagateAcrossPackages covers TS-F02's acceptance
// criterion: sparse pins, dense enforcement, across a package boundary.
//
// Goal: a pinned function whose effect violation is introduced in a
// different package of the module produces the blocking finding at the
// pin, naming the introducing call, and the run exits 1.
func TestCheckFactsPropagateAcrossPackages(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/facts", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "core/core.go:10:1: TS-F02: this function makes a network call "+
		"(fixture.example/facts/helper.Ping at core.go:12:20) but its //tiger:effects "+
		"comment doesn't list io(net) — add io(net) to the comment, or remove the call\n"+
		"tiger: 1 blocking\n", got.stdout)
}

// TestCheckBlockingFindingsExitOne covers the findings contract.
//
// Goal: blocking findings print position-sorted with relative paths, each
// naming its rule ID and the compliant form, and the run exits 1.
func TestCheckBlockingFindingsExitOne(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/findings", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "alpha.go:7:3: TS-S18: panic is called directly here — use assert.Ok "+
		"(condition), assert.Fail (formatted failure), or assert.Unreachable (impossible "+
		"arm) instead so every crash goes through one path\n"+
		"beta.go:10:5: TS-S09: this labeled break jumps out of an inner loop to an outer "+
		"one — move the inner loop into its own function and return instead\n"+
		"tiger: 2 blocking\n", got.stdout)
}

// TestCheckSkipsGeneratedFiles covers the driver's generated-file policy.
//
// Goal: a violation inside a "Code generated ... DO NOT EDIT." file is never
// reported — nothing there is hand-fixable — while violations in the
// package's hand-written files still fire.
func TestCheckSkipsGeneratedFiles(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/generated", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "hand.go:7:3: TS-S18: panic is called directly here — use assert.Ok "+
		"(condition), assert.Fail (formatted failure), or assert.Unreachable (impossible "+
		"arm) instead so every crash goes through one path\n"+
		"tiger: 1 blocking\n", got.stdout)
}

// TestCheckOutputIsDeterministic covers correctness constraint 4.
//
// Goal: the same tree produces byte-identical output across runs.
func TestCheckOutputIsDeterministic(t *testing.T) {
	first := run(t, "check", "-C", "testdata/fixtures/findings", "./...")
	second := run(t, "check", "-C", "testdata/fixtures/findings", "./...")
	assert.Equal(t, first, second)
}

// TestCheckEscapeCountsAgainstBudget covers "escapes are never silent"
// under the ratchet.
//
// Goal: a well-formed //tiger:batched escape in a module with no budget
// row fails the run with a TS-D06 missing-row line, the escape printed as
// a blocking line under it, and the same output on every run.
func TestCheckEscapeCountsAgainstBudget(t *testing.T) {
	want := "tiger.budget.yaml: TS-D06: . has 1 escape directive and tiger.budget.yaml has " +
		"no row for it — fix it, or run tiger budget --write to record the current count\n" +
		"notify.go:7:2: TS-L09: //tiger:batched " +
		"\"provider offers no bulk endpoint; contract caps us at 10 rps\" waives a rule " +
		"here; tiger cannot check the claim, so this notice stands on every run — remove " +
		"the directive when the constraint no longer holds\n" +
		"tiger: 2 blocking\n"

	got := run(t, "check", "-C", "testdata/fixtures/escape", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, want, got.stdout)

	again := run(t, "check", "-C", "testdata/fixtures/escape", "./...")
	assert.Equal(t, cli.ExitFindings, again.code)
	assert.Equal(t, want, again.stdout)
}

// TestCheckSkippedTestCountsAgainstBudget covers TS-D07's reporting
// contract under the ratchet.
//
// Goal: a skipped test in a module with no budget row fails the run with a
// TS-D06 missing-row line and the skip printed under it; the [advisory]
// marker and the advisory trailer count are gone.
func TestCheckSkippedTestCountsAgainstBudget(t *testing.T) {
	want := "tiger.budget.yaml: TS-D06: . has 1 skipped test and tiger.budget.yaml has no " +
		"row for it — fix it, or run tiger budget --write to record the current count\n" +
		"skipped_test.go:11:2: TS-D07: this test is skipped, so it passes without running " +
		"— remove the Skip call when the test can run again; this notice stands until " +
		"then\n" +
		"tiger: 2 blocking\n"

	got := run(t, "check", "-C", "testdata/fixtures/skipped", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, want, got.stdout)
	assert.NotContains(t, got.stdout, "advisory")
}

// TestCheckLoadFailureExitsTwo covers correctness constraint 5.
//
// Goal: a package that fails to load exits 2 — not 0 and not 1 — and the
// failure is reported on stderr, never as a clean run.
func TestCheckLoadFailureExitsTwo(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/broken", "./...")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "packages failed to load")
}

// TestUnknownCommandExitsTwo covers subcommand dispatch.
//
// Goal: an unknown subcommand is an operational error carrying usage, and
// help exits clean.
func TestUnknownCommandExitsTwo(t *testing.T) {
	unknown := run(t, "frobnicate")
	assert.Equal(t, cli.ExitOperational, unknown.code)
	assert.Contains(t, unknown.stderr, "unknown command")
	require.Contains(t, unknown.stderr, "usage:")

	helped := run(t, "help")
	assert.Equal(t, cli.ExitClean, helped.code)
	assert.Contains(t, helped.stdout, "tiger check")

	bare := run(t)
	assert.Equal(t, cli.ExitOperational, bare.code)
	assert.Contains(t, bare.stderr, "usage:")
}
