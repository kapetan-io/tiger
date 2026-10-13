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
// delete, and a boolean short-circuit) stays silent, a collect-then-sort
// passes only when the sort fixes the order, and a maps.Keys, maps.Values
// or maps.All result fires unless it is sorted, collected into a map, or
// ranged over like a map. The five documented known misses stay silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), maporder.Analyzer, "keys", "ts-t02")
}
