package nogoroutine_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/nogoroutine"
)

// TestCorpus runs the TS-C02 and TS-C09 corpus through the analysistest
// driver.
//
// Goal: bare goroutine spawns fire as TS-C02, reactive spawns inside a
// loop fire as TS-C09 instead, and work submitted through a Go method
// stays silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nogoroutine.Analyzer, "ts-c02", "ts-c09")
}
