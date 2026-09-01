package restrictions_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/restrictions"
)

// TestCorpus runs the TS-P01 and TS-P02 corpora through the analysistest
// driver.
//
// Goal: a no-reflect claim contradicted by a reflect import fires, an
// import-list claim contradicted by a disallowed import fires, a second
// //tiger:restrict directive in the same package fires naming the
// duplication, allowed imports and the standard library stay silent, and
// TS-P02 reports the weakest transitive dependency per claimed axis a
// package's own imports do not themselves defend.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), restrictions.Analyzer, "ts-p01", "ts-p02")
}
