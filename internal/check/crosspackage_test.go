package check_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kapetan-io/tiger/internal/command"
)

// TestCheckLedgerExampleIsClean covers acceptance criterion 1's baseline.
//
// Goal: patterns/ledger — every invariant asserted in production and
// violated by a test — exits 0 with no output once the whole-program
// rules register.
func TestCheckLedgerExampleIsClean(t *testing.T) {
	got := run(t, "check", "-C", "../../patterns/ledger", "./...")
	assert.Equal(t, command.ExitClean, got.code)
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
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "unasserted/inv/inv.go:9:2: TS-A07: invariant inv.HeaderSize is declared "+
		"but no function outside _test.go files asserts it — add "+
		"assert.Invariant(inv.HeaderSize, ...) where the property is established, or delete "+
		"the declaration — see `tiger pattern ledger`\n"+
		"tiger: 1 blocking\n", got.stdout)
}

// TestCheckSingleProductionAssertionSatisfiesA07 covers TS-A07's
// threshold: one production function, not two.
//
// Goal: a ledger that asserts inv.HeaderSize only in DecodeHeader exits 0.
func TestCheckSingleProductionAssertionSatisfiesA07(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/invariants", "./singlesite/...")
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestCheckUnviolatedInvariantFailsA09 covers TS-A09 across packages.
//
// Goal: a ledger with no test calling assert.Violates on inv.HeaderChecksum
// exits 1 with one TS-A09 finding at the const.
func TestCheckUnviolatedInvariantFailsA09(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/invariants", "./unviolated/...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "unviolated/inv/inv.go:8:2: TS-A09: no test violates invariant "+
		"inv.HeaderChecksum — add a _test.go function that calls "+
		"assert.Violates(inv.HeaderChecksum, func() { ... }) — see `tiger pattern ledger`\n"+
		"tiger: 1 blocking\n", got.stdout)
}

// TestCheckWholeProgramOutputIsDeterministic covers state invariant 3 on
// a fixture that exercises every whole-program rule.
//
// Goal: two runs over the plugin-smoke fixture — which fires TS-A07 and
// TS-A09 through the finish step — are byte-identical.
func TestCheckWholeProgramOutputIsDeterministic(t *testing.T) {
	first := run(t, "check", "-C", "../../testdata/plugin-smoke", "./...")
	second := run(t, "check", "-C", "../../testdata/plugin-smoke", "./...")
	assert.Equal(t, command.ExitFindings, first.code)
	assert.Equal(t, first, second)
}

// TestCheckPluginSmokeFixtureFiresWholeProgramRules covers acceptance
// criterion 7's CLI half.
//
// Goal: tiger check on the plugin-smoke fixture exits 1 and
// reports TS-A07 and TS-A09 — so the plugin run's silence on those codes in
// CI is meaningful, not vacuous — alongside the findings the plugin does
// report.
func TestCheckPluginSmokeFixtureFiresWholeProgramRules(t *testing.T) {
	got := run(t, "check", "-C", "../../testdata/plugin-smoke", "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	for _, code := range []string{"TS-A07", "TS-A09", "TS-S09"} {
		assert.Contains(t, got.stdout, code+":")
	}
}
