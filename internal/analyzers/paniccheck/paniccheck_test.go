package paniccheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/paniccheck"
)

// TestCorpus runs the TS-S18 corpus through the analysistest driver.
//
// Goal: naked panics fire, assert-routed crashes and shadowed identifiers
// stay silent, and the assert package plus its external test package keep
// their sanctioned exemption.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), paniccheck.Analyzer, "ts-s18", "assert")
}
