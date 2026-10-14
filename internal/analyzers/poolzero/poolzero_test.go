package poolzero_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/poolzero"
)

// TestCorpus runs the TS-M05 corpus through the analysistest driver.
//
// Goal: a Put reached on some path without a zeroing, a Put after only a
// Reset call, and a Put by value all fire TS-M05; `*x = T{}` immediately
// before Put, on every branch that reaches Put, and right after Get all stay
// silent; the documented known-misses — a
// reset and its Put split across a function boundary, and an aliased
// pooled value — stay silent too.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), poolzero.Analyzer, "ts-m05")
}
