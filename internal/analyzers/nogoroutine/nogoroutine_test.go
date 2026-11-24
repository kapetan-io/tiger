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
//
// Goal, TS-C03: only a TestMain calling goleak.VerifyTestMain counts; naming
// another goleak function or calling it inside an `if false` body checks
// nothing (ts-c03verify, ts-c03dead).
//
// Goal, TS-C02: a Wait inside a select case on ctx.Done() is the early return
// of a caller that gave up, so it does not count; a goroutine waiting on its
// own group never finishes (waitsOnItself); TryGo needs a Wait on only some
// path after it, since the path where it returned false started nothing
// (tryGoWaited); a return of a non-nil error is the failure path, so it
// needs no Wait; a path ending in panic never returns and needs none either,
// though no corpus case covers panic.
//
// Goal, TS-C02: a function literal is a function. A start inside a literal is
// waited when a Wait follows it on every success path of that literal, or of
// any literal or declaration around it, whoever calls the literal
// (compliant_literal_test.go). A Wait after the start also counts inside a
// literal that runs before the call holding it returns (a literal called
// where it is written, a saved literal that is called, or a t.Run or b.Run
// body that does not call t.Parallel) when that literal waits on every
// success path. A Wait in a literal saved and never called, in a literal
// passed to another function, in a parallel subtest or in t.Cleanup does not
// count (failure_literal_test.go). The goroutine literal of the same group,
// and any literal inside it, is not a place to Wait, because a Wait there
// waits on itself (failure_literal_test.go). A Go inside a literal that runs
// after the outer Wait or alongside it (a parallel subtest, a saved literal
// called later, a deferred or t.Cleanup literal, another group's goroutine),
// a t.Fatal that skips the Wait, and a literal inside a function that returns
// only a non-nil error are documented gaps (knownmiss_literal_test.go).
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nogoroutine.Analyzer,
		"ts-c02", "ts-c03", "ts-c03eg", "ts-c03go", "ts-c03ignore", "ts-c03ok",
		"ts-c03param", "ts-c03tests", "ts-c03verify", "ts-c03dead", "ts-c09")
}
