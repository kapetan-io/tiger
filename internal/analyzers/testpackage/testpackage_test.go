package testpackage_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/testpackage"
)

// TestCorpus runs the TS-T12 corpus through the analysistest driver.
//
// Goal: test files declared in the package under test fire, export_test.go
// included, and a package whose own name ends in _test is internal when a
// non-test file shares that name.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testpackage.Analyzer,
		"ts-t12", "ts-t12/clean", "ts-t12/shared")
}
