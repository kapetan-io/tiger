package closedworld_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/closedworld"
)

// TestCorpus runs the TS-K03 corpus through the analysistest driver.
//
// Goal: a call through a parameter, a struct field, a call result, a Phi
// of two concrete types, or a standard-library interface all fire in a
// package declaring closed-dispatch; the same shapes stay silent once the
// receiver resolves to one concrete type, and stay silent everywhere in a
// package with no closed-dispatch declaration.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), closedworld.Analyzer, "ts-k03", "ts-k03open")
}
