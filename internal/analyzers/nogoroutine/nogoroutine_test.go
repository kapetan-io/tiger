package nogoroutine_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/nogoroutine"
)

// TestCorpus runs the TS-C02, TS-C03 and TS-C09 corpus through the
// analysistest driver.
//
// Goal: bare goroutine spawns and groups nothing waits on fire as TS-C02,
// reactive spawns inside a loop fire as TS-C09 instead, a package that starts
// goroutines with no goleak check fires as TS-C03, and a started goroutine
// whose group is waited on stays silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nogoroutine.Analyzer,
		"ts-c02", "ts-c03", "ts-c03eg", "ts-c03go", "ts-c03ok", "ts-c03param", "ts-c03tests",
		"ts-c03verify", "ts-c09")
}
