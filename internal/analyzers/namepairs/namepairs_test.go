package namepairs_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/namepairs"
)

// TestCorpus runs the TS-N15 corpus through the analysistest driver.
//
// Goal: every declared identifier built from a banned pair half fires,
// naming the approved half, and the approved-half rewrites stay silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), namepairs.Analyzer, "ts-n15")
}

// TestPairsFlag proves the -pairs flag extends the committed table with a
// project-owned pair on top of the built-in ones.
//
// Goal: the flag-added bad half fires while a built-in pair's approved
// half stays silent.
func TestPairsFlag(t *testing.T) {
	require.NoError(t, namepairs.Analyzer.Flags.Set("pairs", "legacy=modern"))
	defer func() {
		require.NoError(t, namepairs.Analyzer.Flags.Set("pairs", ""))
	}()
	analysistest.Run(t, analysistest.TestData(), namepairs.Analyzer, "extendedpairs")
}
