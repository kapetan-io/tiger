package derivation_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/derivation"
)

// TestCorpus runs the TS-S22 corpus through the analysistest driver.
//
// Goal: a stale derivation fires on both the single-declaration and
// grouped-declaration comment forms, a correct derivation and prose that
// does not parse both stay silent, and an unresolvable identifier inside an
// otherwise parseable derivation stays silent as a documented known miss.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), derivation.Analyzer, "ts-s22")
}
