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
//
// Why those are silent: firing would risk a false positive on a blocking rule.
// A value taken straight from Get is never known-clean, so it fires even with
// no reset in the function (getUseAndPutBack). A Reset call is not a zeroing
// (putAfterResetCall). A deferred Put runs where the function exits, so a
// return before the zeroing reaches the Put dirty (deferPutReturnsBeforeZero).
// Only the canonical zeroing stores count; the compliant form is always
// available.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), poolzero.Analyzer, "ts-m05")
}
