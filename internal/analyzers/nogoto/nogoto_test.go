package nogoto_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/nogoto"
)

// TestCorpus runs the TS-S09 corpus through the analysistest driver.
//
// Goal: the failure modes (goto, labeled break, labeled continue) fire and
// the compliant rewrites (plain loops, extracted functions, unlabeled
// branches) stay silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nogoto.Analyzer, "ts-s09")
}
