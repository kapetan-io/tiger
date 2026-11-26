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
//  Goal, TS-C02: a Wait inside a select case on ctx.Done() is the early
// return of a caller that gave up, so it does not count unless the case also
// holds the start (fanOutOnExpiry). A path through that case needs no Wait,
// but only after the start, only when the start is outside the case, and only
// in a function that waits on the group elsewhere (expiredBeforeGo,
// expiredBeforeTryGo, startInExpired, expiredNoWaitAnywhere, and the
// expiredOnlyIdle type, whose Idle method never waits); a goroutine waiting
// on its own group never finishes (waitsOnItself); TryGo needs a Wait on only
// some path after it, since the path where it returned false started nothing
// (tryGoWaited); a return of a non-nil error is the failure path, so it needs
// no Wait (startThenOnlyError), and neither does a path ending in t.Fatal,
// t.Fatalf or t.FailNow, which fails the test (compliant_fatal_test.go), but
// a function that loops forever or returns only a non-nil error counts as a
// Wait only when code it runs before returning may wait on the group: a Wait
// called directly or through a local bound to the Wait method value, a
// same-package function that may wait on it, a literal it runs (including
// through sync.Once.Do), a same-package function it hands the group that may
// wait on it, or a call it cannot follow that it hands the group, but not a
// builtin or a conversion (failure_nowait.go, failure_nowait_more.go,
// compliant_nowait.go, compliant_handoff.go); such a function whose Wait sits
// on one branch or in a defer is a documented gap (knownmiss.go, ENG-980),
// and so are a Wait after a call that never returns, a Wait in a sync.Once
// that already fired, and a group handed to a call it cannot follow that
// never waits (knownmiss.go); a saved literal there counts only when every
// literal that may reach the call waits, so one called on a loop's first pass
// and replaced after it is an expected false positive
// (failure_nowait_more.go, ENG-1045); a path ending in panic never returns
// and needs none either, though no corpus case covers panic.
//
// Goal, TS-C02: a function literal is a function. A start inside a literal is
// waited when a Wait follows it on every success path of that literal, or of
// any literal or declaration around it, whoever calls the literal
// (compliant_literal_test.go). A Wait after the start also counts inside a
// literal that runs before the call holding it returns (a literal called
// where it is written, a call to a local variable that is never reached
// through & or rebound by a range clause and whose only definition is a
// literal, or that is declared in one function body, assigned only there or
// in a literal it defers, and whose last assignment on the path to that call
// is a literal, with the call in that body, in a literal called where it is
// written, or in a subtest body that counts; or a t.Run or b.Run body that
// does not call t.Parallel or is started on the t of a subtest whose body
// counts, from inside that subtest's body and through no variable declared
// outside it) when that literal waits on every success path. A literal that
// returns an error before its Wait counts only on the caller's branch where
// that error is nil, or when the caller returns the call's result; a caller
// that drops the error or goes on after it is reported
// (compliant_literal_test.go, failure_literal_test.go). A Wait in a literal
// saved and never called, in a literal passed to another function, in a saved
// literal that another assignment may replace on a path to the call that has
// not waited yet, in a parallel subtest started on any other t or in
// t.Cleanup does not count (failure_literal_test.go). A saved literal called
// from a nested literal in a loop that replaces it after the first pass is
// reported as an expected false positive (ENG-1043), and so is the same loop
// after four other followed variables. The goroutine
// literal of the same group, and any literal inside it, is not a place to
// Wait, because a Wait there waits on itself (failure_literal_test.go). A Go
// inside a literal that runs after the outer Wait or alongside it (a parallel
// subtest, a saved literal called later, a deferred or t.Cleanup literal,
// another group's goroutine), a t.Fatal that skips the Wait, and a literal
// inside a function that returns only a non-nil error are documented gaps
// (knownmiss_literal_test.go), and so
// are a t.Fatal or t.Skip that skips the Wait outside any subtest and a helper
// that fails the test on some of its paths (knownmiss_fatal_test.go).
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nogoroutine.Analyzer,
		"ts-c02", "ts-c03", "ts-c03eg", "ts-c03go", "ts-c03ignore", "ts-c03ok",
		"ts-c03param", "ts-c03tests", "ts-c03verify", "ts-c03dead", "ts-c09")
}
