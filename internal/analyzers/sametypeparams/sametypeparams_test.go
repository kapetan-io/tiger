package sametypeparams_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/sametypeparams"
)

// TestCorpus runs the TS-N07 and TS-N08 corpus through the analysistest
// driver.
//
// Goal: adjacent same-type parameters and over-four-parameter lists fire
// TS-N07, plain bool parameters fire TS-N08, the options-struct and
// named-type compliant rewrites stay silent, the documented known miss
// stays silent, and the assert support package keeps its exemption.
func TestCorpus(t *testing.T) {
	analysistest.Run(
		t,
		analysistest.TestData(),
		sametypeparams.Analyzer,
		"ts-n07",
		"ts-n08",
		"assert",
	)
}
