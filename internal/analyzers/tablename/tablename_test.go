package tablename_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/tablename"
)

// TestCorpus runs the TS-T10 corpus through the analysistest driver.
//
// Goal: an anonymous-struct table without a name field fires inside
// Test/Benchmark/Fuzz functions, a name field (either case) or a
// map[string]struct table stays silent, and a named struct type table is a
// documented known miss.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), tablename.Analyzer, "ts-t10")
}
