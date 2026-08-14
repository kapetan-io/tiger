package boundedloop_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/boundedloop"
)

// TestCorpus runs the TS-S02 and TS-S03 corpora through the analysistest
// driver.
//
// Goal: unbounded for{}, non-derivable conditions, and channel ranges fire
// TS-S02; a select missing a ctx.Done() case fires TS-S03; every compliant
// rewrite stays silent; documented known-miss cases stay silent too.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), boundedloop.Analyzer, "ts-s02", "ts-s03")
}
