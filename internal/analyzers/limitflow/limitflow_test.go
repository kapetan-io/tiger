package limitflow_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/limitflow"
)

// TestCorpus runs the TS-S02 limit corpus through the analysistest driver.
//
// Goal: a value that enters a loop limit with no bound, or a constant no loop
// reaches, fires; a constant, a length, a clamp against a declared maximum,
// and a helper that returns a clamped value stay silent; the facts the trace
// exports across packages match what the corpus expects.
//
// Rules the corpus pins, by fixture file:
//   - A clamp caps only the upper end, so subtracting, negating, multiplying,
//     dividing by a negative or bit-clearing a clamped value is unbounded
//     (failure_negative_else.go); wrap-around is a known miss
//     (knownmiss_overflow.go).
//   - A clamp is undone by a later write, a closure write to the same variable
//     or field chain, a write through a pointer or a call that receives the
//     struct, a loop that replaces the value, or an else branch that writes it
//     (failure_clamp_undone.go, failure_loop_limit_gaps*.go,
//     failure_trace_gaps.go, failure_negative_else.go). The closure check
//     ignores run order, so a closure that runs before the clamp is a
//     documented false positive (failure_closure_order.go).
//   - A check on a narrowing conversion such as int8(x) says nothing about x,
//     while int(req.Limit) matches a use spelled the same way
//     (failure_clamp_undone.go, compliant.go).
//   - A variable that feeds itself other than by v += x (doubling) never
//     settles and counts as unbounded; v += x follows x alone, so adding a
//     constant in a loop is a recorded miss (failure_accumulator.go,
//     knownmiss_accumulator.go).
//   - Values read in function literals are not judged, nor is a _test.go
//     file's table data (compliant_shapes.go,
//     compliant_tests_are_not_traced_test.go); a header read of a field of an
//     unnamed struct is trusted and exports no fact.
//   - A call through a function value or an interface method is not followed,
//     a recorded miss (knownmiss.go, knownmiss_more.go).
//   - Constants at least half the type's maximum are unreachable only on
//     types wide enough that a loop cannot count to them
//     (failure_wide_types.go, compliant_narrow_types.go).
//   - Packages from the standard library are never judged and export no facts;
//     the run includes "strings", which would report if it were judged.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), limitflow.Analyzer,
		"lib", "types", "store", "ts-s02", "strings")
}
