package nogoroutine_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/nogoroutine"
)

// TestCorpus runs the TS-C02 and TS-C09 corpus through the analysistest
// driver.
//
// Goal: bare goroutine spawns fire as TS-C02 outside a supervisor,
// reactive spawns inside a loop fire as TS-C09 instead, and both the
// allowlisted function and the allowlisted method stay silent.
func TestCorpus(t *testing.T) {
	err := nogoroutine.Analyzer.Flags.Set("supervisors", "ts-c02.Supervise,ts-c02.Runner.Run")
	require.NoError(t, err)
	analysistest.Run(t, analysistest.TestData(), nogoroutine.Analyzer, "ts-c02", "ts-c09")
}
