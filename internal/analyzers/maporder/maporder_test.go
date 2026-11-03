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
//
// Traps the corpus pins:
//   - A sort proves nothing if the slice can be read under another name
//     first (copied, passed, returned, captured, sliced, address taken), if a
//     break, continue, goto or bare return of a named result can skip the sort,
//     or if the body does more than collect.
//   - A body that can break or return must change nothing but constant
//     booleans and per-visit locals, and every exit must carry every outcome
//     the body can leave.
//   - A write through a pointer field may be read through another entry that
//     shares the pointer; slice and array element writes are refused because
//     an index can name a slot another visit also writes.
//   - A comparator is accepted only if it returns 0 for equal elements alone:
//     every leaf must be covered by an integer, string or comparable whole
//     array, and floats never are because -0 and +0 compare equal but print
//     differently.
//   - A slot-filling pair `s[i] = e; i++` is accepted only when nothing sits
//     between the store and the step, or the next visit overwrites the slot.
//   - Search budgets (maxPathDepth, maxCoverDepth, maxCoverArray,
//     maxCoverParts) report the sort when exceeded, never accept it; only
//     maxCoverParts is exceeded by a corpus case (collectBigPartial).
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), maporder.Analyzer, "keys", "ts-t02")
}
