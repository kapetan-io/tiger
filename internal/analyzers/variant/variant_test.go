package variant_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/variant"
)

// TestCorpus runs the TS-V01 corpus through the analysistest driver.
//
// Goal: a loop with no linear ranking, a conditional-only decrease, a
// continue that can skip the decrease, a recognized condition paired with
// an unrecognized decrease, and an unverifiable pin all fire TS-V01; a pin
// on a loop that needs no variant fires TS-V01 too; every out-of-scope and
// verified-pin shape stays silent; every synthesizable loop fires the
// TS-V01-facts reported finding in exact pin syntax.
//
// Traps the corpus pins:
//   - A pin rides on a condition term only when it names the same variables
//     the term compares and every one the loop changes, with the same sign:
//     a pin on n cannot ride on a condition about i and m, nor stand alone
//     for i < n when the loop also moves i.
//   - A division or shift shrinks only a value known to be above zero, and a
//     slice cut by len(part) needs part shown not empty.
//   - A mover that can wrap past the end of its integer type before meeting
//     the limit never ends; a variable or field also written through a
//     pointer or closure made before the loop, or a struct passed to a call,
//     is unknown.
//   - A continue in a nested loop ends only that loop's pass, but a labeled
//     continue to a label outside the body, or a goto, can skip the move.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), variant.Analyzer, "ts-v01")
}
