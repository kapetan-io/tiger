package declusedistance_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/declusedistance"
)

// TestCorpus runs the TS-S13 corpus through the analysistest driver.
//
// Goal: a declaration separated from its first use by more than 10 lines
// fires TS-S13, whether the use is a plain assignment, read, or a bare
// method call; a declaration at its point of use or exactly 10 lines from
// it (the boundary) stays silent; and a declaration whose first reference
// is a nearby closure stays silent even when the actual identifier use
// deep inside that closure is far away, since distance is never measured
// across a frame boundary — the known-miss case documents what that hides.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declusedistance.Analyzer, "ts-s13")
}
