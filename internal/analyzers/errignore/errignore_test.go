package errignore_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/errignore"
)

// TestCorpus runs the TS-E02 corpus through the analysistest driver.
//
// Goal: an error discard fires with or without a comment beside it, and a
// non-error or range discard never triggers the rule.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), errignore.Analyzer, "ts-e02")
}
