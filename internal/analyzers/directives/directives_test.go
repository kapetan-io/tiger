package directives_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/directives"
)

// TestCorpus runs the TS-L09 corpus through the analysistest driver.
//
// Goal: unknown verbs, reasonless escapes, and trackerless skips fire as
// blocking findings; a well-formed escape fires the standing advisory; pins,
// intent declarations, and prose stay silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), directives.Analyzer, "ts-l09")
}
