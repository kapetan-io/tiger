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
// each fire one TS-S01 finding at the cycle's lexically first member. So do
// the same shapes written with type parameters: a generic self-call, generic
// mutual recursion, a plain and a generic function calling each other, a
// method on a generic type, a closure in a generic function, and a call
// through an instantiated function value. Every iterative rewrite, and a
// generic function that does not recurse, stays silent; the documented
// known-miss cases (interface dispatch, an unresolved function value) stay
// silent too. Two methods that share an unqualified name are ordered by
// source position so the message stays deterministic; no corpus case
// collides names, so that is unpinned.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), norecursion.Analyzer, "ts-s01")
}
