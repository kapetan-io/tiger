package limitrelate_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/limitrelate"
)

// TestCorpus runs the TS-S21 corpus through the analysistest driver.
//
// Goal: an unrelated Max/Min constant fires; a constant related through a
// blank-named uint relational assertion and used outside it, and an
// out-of-scope local constant, both stay silent. An unexported constant
// mentioned only inside such assertions fires. A tautological self-cast
// assertion, a companion with another name and an exported guard-only
// constant stay silent as documented known misses.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), limitrelate.Analyzer, "ts-s21")
}
