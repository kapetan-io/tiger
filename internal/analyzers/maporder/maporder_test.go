package maporder_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/maporder"
)

// TestCorpus runs the TS-T02 corpus through the analysistest driver.
//
// Goal: every range over a map whose body falls outside the closed
// order-insensitive allowlist fires TS-T02, every allowlisted shape (map
// writes, integer accumulation, integer min/max in both forms,
// delete, and a boolean short-circuit) stays silent, and the sorted-keys
// rewrite the finding names ranges over a slice instead of a map and so is
// never inspected. The documented known-miss (order escaping through an
// unsorted key collection that never ranges over a map) stays silent too.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), maporder.Analyzer, "ts-t02")
}
