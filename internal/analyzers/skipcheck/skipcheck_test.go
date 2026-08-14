package skipcheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/skipcheck"
)

// TestCorpus runs the TS-D07 corpus through the analysistest driver.
//
// Goal: every skip spelling on every testing receiver is reported, tests
// that run and domain Skip methods stay silent, and the embedded-receiver
// known miss holds.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), skipcheck.Analyzer, "ts-d07")
}
