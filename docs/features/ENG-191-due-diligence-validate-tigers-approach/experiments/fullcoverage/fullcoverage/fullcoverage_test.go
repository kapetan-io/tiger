package fullcoverage_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"fullcoverage/fullcoverage"
)

// TestCorpus proves the prototype's verdict on every corpus case: tiger's
// TS-T02 cases keep their verdict (except the slices.Collect known miss,
// now caught), each soundness boundary of full coverage is rejected, the
// shapes it is meant to accept pass, the total-compare fact crosses
// packages, and the expected false positives still fire.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), fullcoverage.Analyzer, "keys", "ts-t02")
}
