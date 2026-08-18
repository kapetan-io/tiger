package selectctx_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/selectctx"
)

// TestCorpus runs the TS-C05 corpus through the analysistest driver.
//
// Goal: a blocking select with no shutdown case fires, a blocking send or
// receive outside any select fires, the compliant rewrites — including a
// recognized struct{}-element or shutdown-named channel case, and a bare
// receive or send on one — stay silent, and the known misses (channel
// range, a false shutdown name-match, and a non-shutdown struct{}
// semaphore) hold.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), selectctx.Analyzer, "ts-c05")
}
