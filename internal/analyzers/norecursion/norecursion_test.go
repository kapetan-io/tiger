package norecursion_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/norecursion"
)

// TestCorpus runs the TS-S01 corpus through the analysistest driver.
//
// Goal: direct self-recursion, mutual recursion, method recursion on a
// concrete type, and recursion through a statically-known function literal
// each fire one TS-S01 finding at the cycle's lexically first member; every
// iterative rewrite stays silent; the documented known-miss cases (interface
// dispatch, an unresolved function value) stay silent too.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), norecursion.Analyzer, "ts-s01")
}
