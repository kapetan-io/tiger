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
// each fire one TS-S01 finding at the cycle's lexically first member (first by file name, then line); every
// iterative rewrite stays silent; the documented known-miss cases (interface
// dispatch, an unresolved function value) stay silent too. Two methods that
// share an unqualified name are ordered by file name, then line, so the message
// stays deterministic; no corpus case collides names, so that is unpinned.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), norecursion.Analyzer, "ts-s01")
}

// TestCycleAcrossFiles runs a cycle whose members sit in two files. The
// package loader parses files in parallel, so a later file can get lower
// token.Pos values than an earlier one, and that order varies between loads; the finding must land at the first member by file
// name on every load, so the test loads the package repeatedly.
func TestCycleAcrossFiles(t *testing.T) {
	for range 20 {
		analysistest.Run(t, analysistest.TestData(), norecursion.Analyzer, "ts-s01-twofiles")
	}
}
