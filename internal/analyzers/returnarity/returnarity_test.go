package returnarity_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/returnarity"
)

// TestCorpus runs the TS-E06 corpus through the analysistest driver.
//
// Goal: too-many-result and bad-second-result failure modes fire across
// func declarations, func literals, and interface methods, while the
// compliant zero, one, (T, error), and (T, bool) shapes stay silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), returnarity.Analyzer, "ts-e06")
}
