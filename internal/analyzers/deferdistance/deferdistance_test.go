package deferdistance_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/deferdistance"
)

// TestCorpus runs the TS-L10 corpus through the analysistest driver.
//
// Goal: a defer inside a loop fires TS-L10, a defer placed behind an
// unrelated statement or in a different block from its acquisition fires
// TS-L10-distance, defers next to their acquisition (immediate or after the
// standard err check, or outside any loop) stay silent, and a deferred
// closure or package call is a documented known miss for the distance half.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), deferdistance.Analyzer, "ts-l10")
}
