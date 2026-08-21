package poolzero_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/poolzero"
)

// TestCorpus runs the TS-M05 corpus through the analysistest driver.
//
// Goal: a Put reached on some path without a reset, and a Put of a type
// with no Reset method, both fire TS-M05; a reset immediately before Put, a
// reset present on every branch that reaches Put, and the `*x = T{}`
// zeroing-store form all stay silent; the documented known-misses — a
// reset and its Put split across a function boundary, and an aliased
// pooled value — stay silent too.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), poolzero.Analyzer, "ts-m05")
}
