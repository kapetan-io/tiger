package namedeny_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/namedeny"
)

// TestCorpus runs the TS-N12 and TS-N13 corpus through the analysistest
// driver.
//
// Goal: deny-dictionary and type-echo failure modes fire, their compliant
// rewrites stay silent, the assert package stays exempt, and the range-loop
// known miss stays documented and silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), namedeny.Analyzer, "ts-n12", "ts-n13", "assert")
}

// quietRun swallows analysistest's expectation-mismatch reporting so this
// test can inspect the returned Result directly instead of the pass/fail
// output analysistest normally writes to a *testing.T.
type quietRun struct{}

func (quietRun) Errorf(string, ...any) {}

// TestAllowFlag proves the -allow flag silences exactly the tokens it
// names, leaving the rest of the TS-N12 dictionary enforced.
//
// Goal: an allowlisted token stays silent while an unlisted token in the
// same package still fires.
func TestAllowFlag(t *testing.T) {
	require.NoError(t, namedeny.Analyzer.Flags.Set("allow", "helper=legacy vendor API name"))
	defer func() {
		require.NoError(t, namedeny.Analyzer.Flags.Set("allow", ""))
	}()
	analysistest.Run(t, analysistest.TestData(), namedeny.Analyzer, "allowlisted")
}

// TestAllowFlagRejectsMissingReason proves a malformed -allow entry (no
// "=reason" half) is a configuration error, not a silently ignored token.
//
// Goal: run returns an error instead of a diagnostic when -allow is
// malformed.
func TestAllowFlagRejectsMissingReason(t *testing.T) {
	require.NoError(t, namedeny.Analyzer.Flags.Set("allow", "helper"))
	defer func() {
		require.NoError(t, namedeny.Analyzer.Flags.Set("allow", ""))
	}()
	results := analysistest.Run(
		quietRun{},
		analysistest.TestData(),
		namedeny.Analyzer,
		"allowlisted",
	)
	require.Len(t, results, 1)
	assert.ErrorContains(t, results[0].Err, "needs a reason")
}
