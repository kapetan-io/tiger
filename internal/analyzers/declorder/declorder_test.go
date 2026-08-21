package declorder_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/declorder"
)

// TestCorpus runs the TS-L05 corpus through the analysistest driver.
//
// Goal: a struct's method or constructor declared ahead of something it
// must follow fires TS-L05; correct order, a constructor-less struct, and
// cross-file placement (a documented known miss) all stay silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declorder.Analyzer, "ts-l05")
}
