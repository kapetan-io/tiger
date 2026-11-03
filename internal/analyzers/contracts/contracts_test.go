package contracts_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/contracts"
)

// TestCorpus runs the TS-V03 corpus through the analysistest driver.
//
// Goal: a literal nil, a zero integer constant, and a wrong-size composite
// literal each prove a requires violation at their call site; a
// constant-negative return proves an ensures violation; every compliant
// call and return stays silent; and the documented known-miss cases —
// flow-only nil, a bare invariant ID, a cross-package callee — stay silent
// too. A naked return stays silent because judging the named results needs
// flow analysis. A call spreading arguments with "..." is skipped because
// the spread breaks the positional match between parameters and arguments.
// A return inside a nested function literal is never attributed to the
// enclosing function.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), contracts.Analyzer, "ts-v03")
}
