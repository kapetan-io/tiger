package ioinloop_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/directives"
	"github.com/kapetan-io/tiger/internal/analyzers/ioinloop"
)

// TestCorpus runs the TS-M10 corpus through the analysistest driver.
//
// Goal: stdlib IO resolved through pass.TypesInfo to a package on the
// allowlist fires inside a for or range body; a //tiger:batched loop, the
// same IO hoisted above the loop, IO outside any loop, and an in-memory
// bytes.Buffer write all stay silent; an inner loop under an
// outer-only-annotated directive still fires (the nested-loop exclusion);
// an unlisted package, a same-package helper call, and IO inside a FuncLit
// defined in the loop stay silent as documented known misses.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ioinloop.Analyzer, "ts-m10")
}

// TestEscapeStandingAdvisory proves ioinloop's consumption of
// //tiger:batched does not couple to the directives analyzer's reporting:
// the same annotated loop that silences TS-M10 still carries the
// TS-L09-escape standing advisory under a separate analyzer run.
//
// Goal: the escape directive on an ioinloop-waived loop still fires
// TS-L09-escape when run through the directives analyzer.
func TestEscapeStandingAdvisory(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), directives.Analyzer, "ts-m10-escape")
}
