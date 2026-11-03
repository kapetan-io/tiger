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
//
// Reasons the corpora pin, so a case is not "simplified" away:
//   - x != 0 is bounded only for integer counters moved by a right shift
//     (unsigned only: a signed shift stops at -1) or a division by a constant
//     above 1. Float division never moves NaN or Inf, x-- can wrap past zero
//     from a negative start, and a shift by a variable may shift by zero.
//   - Every assignment to x in the loop must move it toward zero, and one must
//     run each iteration; an assignment under an if, or after a continue or
//     goto, may be skipped. Counters match by object identity, so a shadowing
//     := neither helps nor hurts, and a closure body counts neither way.
//   - Negation flips the rules (De Morgan): !(A && B) is !A || !B, so && is
//     "either operand bounded" only when not negated. !(x != 0) runs while x is
//     zero, so the != 0 and shrinking-gap (i < j) widenings never apply there.
//   - The shrinking-gap proof takes integer i and j only (a float step of +1
//     can be absorbed) and requires that neither is reassigned in the body.
//   - A select with a default clause is a non-blocking drain loop and needs no
//     stop case; a stop case that falls back into the loop stops nothing.
//   - TS-S23: a return inside a function literal, or an unlabeled break inside
//     a nested select, switch or loop, leaves that statement, not the event
//     loop; a case on a send-only channel counts as a stop case only when it
//     ends in a return; a goto to the loop's own label restarts it.
//   - A cap on a rows.Next()/Scan() style loop is exempt only for a page size
//     over a stream declared outside the module; other boolean methods say
//     nothing about a stream running dry.
//   - A counter cap with another exit must report when hit (assert, panic or
//     error return); an assert or panic call right after the loop is trusted.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), boundedloop.Analyzer, "ts-s02", "ts-s03", "ts-s23")
}
