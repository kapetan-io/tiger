package declorder_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/declorder"
)

// TestCorpus runs the TS-N06 and TS-L05 corpora through the analysistest
// driver.
//
// Goal: an unexported, receiver-less helper with exactly one certain
// caller fires TS-N06 when its name doesn't carry that caller's prefix; a
// second caller, an uncertain call set, or an exempt caller name (main,
// init, a test function) all stay silent. A struct's method or constructor
// declared ahead of something it must follow fires TS-L05; correct order,
// a constructor-less struct, and cross-file placement (a documented known
// miss) all stay silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declorder.Analyzer, "ts-n06", "ts-l05")
}
