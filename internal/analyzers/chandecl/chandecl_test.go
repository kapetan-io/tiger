package chandecl_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/chandecl"
)

// TestCorpus runs the TS-C12 corpus through the analysistest driver.
//
// Goal: a channel-carrying type declared outside the package's canonical
// file fires, declarations colocated in the canonical file stay silent, and
// anonymous channel types in signatures or var declarations never count.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), chandecl.Analyzer, "ts-c12")
}
