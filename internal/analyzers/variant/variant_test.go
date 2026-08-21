package variant_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/variant"
)

// TestCorpus runs the TS-V01 corpus through the analysistest driver.
//
// Goal: a loop with no linear ranking, a conditional-only decrease, a
// continue that can skip the decrease, a recognized condition paired with
// an unrecognized decrease, and an unverifiable pin all fire TS-V01; a pin
// on a loop that needs no variant fires TS-V01 too; every out-of-scope and
// verified-pin shape stays silent; every synthesizable loop fires the
// TS-V01-facts reported finding in exact pin syntax.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), variant.Analyzer, "ts-v01")
}
