package errignore_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/errignore"
)

// TestCorpus runs the TS-E02 corpus through the analysistest driver.
//
// Goal: an uncommented error discard fires, a commented discard (trailing
// or on the line above) stays silent, and a non-error or range discard
// never triggers the rule.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), errignore.Analyzer, "ts-e02")
}
