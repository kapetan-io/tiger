package compoundcond_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/compoundcond"
)

// TestCorpus runs the TS-S06, TS-S07, and TS-S08 corpus through the
// analysistest driver.
//
// Goal: mixed &&/|| conditions and unsplit compound assertions fire, closed
// switches missing a proper assert.Unreachable default fire, the compliant
// rewrites of all three stay silent, and the documented known misses hold.
func TestCorpus(t *testing.T) {
	analysistest.Run(
		t,
		analysistest.TestData(),
		compoundcond.Analyzer,
		"ts-s06",
		"ts-s07",
		"ts-s08",
	)
}
