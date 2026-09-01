package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kapetan-io/tiger/internal/cli"
)

// TestCheckLedgerExampleIsClean covers acceptance criterion 1's baseline.
//
// Goal: examples/ledger — every invariant asserted in production and
// violated by a test — exits 0 with no output once the whole-program
// rules register.
func TestCheckLedgerExampleIsClean(t *testing.T) {
	got := run(t, "check", "-C", "../../examples/ledger", "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestCheckUnassertedInvariantFailsA07 covers TS-A07 across packages.
//
// Goal: a ledger whose header.go asserts inv.HeaderSize nowhere — the
// const is still declared, and a test still violates it — exits 1 with
// one TS-A07 finding positioned at the const in the inv package, not at
// any call site.
func TestCheckUnassertedInvariantFailsA07(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/invariants", "./unasserted/...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "unasserted/inv/inv.go:9:2: TS-A07: invariant inv.HeaderSize is declared "+
		"but no function outside _test.go files asserts it — add "+
		"assert.Invariant(inv.HeaderSize, ...) where the property is established, or delete "+
		"the declaration\n"+
		"tiger: 1 blocking, 0 advisory\n", got.stdout)
}

// TestCheckSingleProductionAssertionSatisfiesA07 covers TS-A07's
// threshold: one production function, not two.
//
// Goal: a ledger that asserts inv.HeaderSize only in DecodeHeader exits 0.
func TestCheckSingleProductionAssertionSatisfiesA07(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/invariants", "./singlesite/...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestCheckUnviolatedInvariantFailsA09 covers TS-A09 across packages.
//
// Goal: a ledger with no test calling assert.Violates on inv.HeaderChecksum
// exits 1 with one TS-A09 finding at the const.
func TestCheckUnviolatedInvariantFailsA09(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/invariants", "./unviolated/...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "unviolated/inv/inv.go:8:2: TS-A09: no test violates invariant "+
		"inv.HeaderChecksum — add a _test.go function that calls "+
		"assert.Violates(inv.HeaderChecksum, func() { ... })\n"+
		"tiger: 1 blocking, 0 advisory\n", got.stdout)
}

// TestCheckSingleImplementationFailsX01 covers acceptance criterion 2.
//
// Goal: an interface implemented by exactly one non-test type exits 1 with
// TS-X01 at the interface naming the implementation; a second
// implementation outside _test.go files makes the run exit 0; a second
// implementation that lives in a _test.go file does not count, so the run
// still exits 1.
func TestCheckSingleImplementationFailsX01(t *testing.T) {
	single := run(t, "check", "-C", "testdata/fixtures/singleimpl", "./single/...")
	assert.Equal(t, cli.ExitFindings, single.code)
	assert.Empty(t, single.stderr)
	assert.Equal(t, "single/storage.go:5:6: TS-X01: interface Storage has one implementation, "+
		"diskStorage — use diskStorage directly and delete the interface, or add a second "+
		"implementation outside _test.go files\n"+
		"tiger: 1 blocking, 0 advisory\n", single.stdout)

	pair := run(t, "check", "-C", "testdata/fixtures/singleimpl", "./pair/...")
	assert.Equal(t, cli.ExitClean, pair.code)
	assert.Empty(t, pair.stdout)

	double := run(t, "check", "-C", "testdata/fixtures/singleimpl", "./double/...")
	assert.Equal(t, cli.ExitFindings, double.code)
	assert.Equal(t, "double/storage.go:6:6: TS-X01: interface Storage has one implementation, "+
		"diskStorage — use diskStorage directly and delete the interface, or add a second "+
		"implementation outside _test.go files\n"+
		"tiger: 1 blocking, 0 advisory\n", double.stdout)
}

// TestCheckWholeProgramOutputIsDeterministic covers state invariant 3 on
// a fixture that exercises every whole-program rule.
//
// Goal: two runs over the plugin-smoke fixture — which fires TS-A07,
// TS-A09, and TS-X01 through the finish step — are byte-identical.
func TestCheckWholeProgramOutputIsDeterministic(t *testing.T) {
	first := run(t, "check", "-C", "../../testdata/plugin-smoke", "./...")
	second := run(t, "check", "-C", "../../testdata/plugin-smoke", "./...")
	assert.Equal(t, cli.ExitFindings, first.code)
	assert.Equal(t, first, second)
}

// TestCheckPluginSmokeFixtureFiresWholeProgramRules covers acceptance
// criterion 7's CLI half.
//
// Goal: tiger check on the two-package plugin-smoke fixture exits 1 and
// reports TS-A07, TS-A09, and TS-X01 — so the plugin run's silence on those
// codes in CI is meaningful, not vacuous — alongside the TS-F02 and TS-P01
// findings the plugin does report.
func TestCheckPluginSmokeFixtureFiresWholeProgramRules(t *testing.T) {
	got := run(t, "check", "-C", "../../testdata/plugin-smoke", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	for _, code := range []string{"TS-A07", "TS-A09", "TS-X01", "TS-F02", "TS-P01"} {
		assert.Contains(t, got.stdout, code+":")
	}
}

// TestCheckRestrictionContradictedByImportsFailsP01 covers acceptance
// criterion 3.
//
// Goal: a package whose //tiger:restrict declares no-reflect and an
// imports list limited to internal/domain, yet imports reflect and a
// module package outside that subtree, exits 1 with two TS-P01 findings,
// one per offending import spec, while its declaring dependencies weaken
// nothing; a package with a stdlib-only import set and no directive exits
// 0.
func TestCheckRestrictionContradictedByImportsFailsP01(t *testing.T) {
	banned := run(t, "check", "-C", "testdata/fixtures/restrict", "./...")
	assert.Equal(t, cli.ExitFindings, banned.code)
	assert.Empty(t, banned.stderr)
	assert.Equal(t, "banned/banned.go:9:2: TS-P01: package banned declares //tiger:restrict "+
		"no-reflect but imports reflect — remove the reflect import or drop no-reflect from the "+
		"directive\n"+
		"banned/banned.go:13:2: TS-P01: package banned imports fixture.example/restrict/other, "+
		"which its //tiger:restrict imports(...) list does not allow — remove the import or "+
		"add the path to imports(...)\n"+
		"tiger: 2 blocking, 0 advisory\n", banned.stdout)

	clean := run(t, "check", "-C", "testdata/fixtures/restrict", "./clean/...")
	assert.Equal(t, cli.ExitClean, clean.code)
	assert.Empty(t, clean.stdout)
	assert.Empty(t, clean.stderr)
}

// TestCheckOpenDispatchUnderClosedDispatchFailsK03 covers acceptance
// criterion 4.
//
// Goal: in a package declaring closed-dispatch, an interface method call
// whose receiver is a parameter exits 1 with TS-K03 at the call; the same
// call with a receiver built from a single concrete value in the same
// function exits 0.
func TestCheckOpenDispatchUnderClosedDispatchFailsK03(t *testing.T) {
	param := run(t, "check", "-C", "testdata/fixtures/closed", "./param/...")
	assert.Equal(t, cli.ExitFindings, param.code)
	assert.Empty(t, param.stderr)
	assert.Equal(t, "param/param.go:14:16: TS-K03: s.Write is called through interface Storage "+
		"in a package that declares //tiger:restrict closed-dispatch — call the concrete type's "+
		"method, or drop closed-dispatch\n"+
		"tiger: 1 blocking, 0 advisory\n", param.stdout)

	local := run(t, "check", "-C", "testdata/fixtures/closed", "./local/...")
	assert.Equal(t, cli.ExitClean, local.code)
	assert.Empty(t, local.stdout)
	assert.Empty(t, local.stderr)
}

// TestCheckWeakenedRestrictionFailsP02PerAxis covers acceptance criterion
// 5 under ADR-0012's ruling that TS-P02 blocks.
//
// Goal: a package claiming closed-dispatch and no-reflect whose two
// dependencies (loaded in the same run, so their declarations are facts)
// each lack a different one of those axes exits 1 with two
// TS-P02 findings at its package clause — one per weakened axis, each
// naming the dependency that weakens it and the edit — and a second run
// prints identical bytes.
func TestCheckWeakenedRestrictionFailsP02PerAxis(t *testing.T) {
	want := "app/app.go:6:1: TS-P02: package app claims closed-dispatch but imports store, " +
		"which does not claim closed-dispatch — add //tiger:restrict closed-dispatch to store, " +
		"or drop closed-dispatch from app's declaration\n" +
		"app/app.go:6:1: TS-P02: package app claims no-reflect but imports codec, which does " +
		"not claim no-reflect — add //tiger:restrict no-reflect to codec, or drop no-reflect " +
		"from app's declaration\n" +
		"tiger: 2 blocking, 0 advisory\n"
	first := run(t, "check", "-C", "testdata/fixtures/weakened", "./...")
	assert.Equal(t, cli.ExitFindings, first.code)
	assert.Empty(t, first.stderr)
	assert.Equal(t, want, first.stdout)

	second := run(t, "check", "-C", "testdata/fixtures/weakened", "./...")
	assert.Equal(t, first, second)
}
