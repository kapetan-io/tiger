package limitflow_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/limitflow"
)

// TestCorpus runs the TS-S02 limit corpus through the analysistest driver.
//
// Goal: a value that enters a loop limit with no bound, or a constant no loop
// reaches, fires; a constant, a length, a clamp against a declared maximum,
// and a helper that returns a clamped value stay silent; the facts the trace
// exports across packages match what the corpus expects.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), limitflow.Analyzer,
		"lib", "types", "store", "ts-s02")
}
