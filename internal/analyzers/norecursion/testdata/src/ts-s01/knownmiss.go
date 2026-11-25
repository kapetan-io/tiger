// Documented TS-S01 coverage gaps: recursion that a real call graph would
// see but this analyzer's static call graph cannot, because the callee is
// not resolvable from the call instruction alone.
//
// known-miss: interface dispatch (the receiver's dynamic type decides the
// callee), a method called on a type-parameter value, an unresolved function
// value (the callee is chosen behind a branch), and a cycle spanning a
// package boundary are all outside the static call graph this analyzer builds
// from ssalib.StaticCalls. None of the four is reported here; each is flagged for spec amendment alongside
// the wave-1 TS-S02 amendment rather than approximated into a maybe-finding.
package fixture

// Stepper is implemented by two types that call each other back through the
// interface, never through a concrete type.
type Stepper interface {
	Step(next Stepper, n int) int
}

// A and B recurse into each other only through the Stepper interface value
// passed in — the receiver's dynamic type is what decides which Step runs.
//
// known-miss: an interface method call's callee depends on the receiver's
// dynamic type, which StaticCalls cannot resolve, so this cycle has no edge
// in the static call graph and stays silent.
type A struct{}

func (A) Step(next Stepper, n int) int {
	if n == 0 {
		return 0
	}
	return next.Step(A{}, n-1)
}

type B struct{}

func (B) Step(next Stepper, n int) int {
	if n == 0 {
		return 0
	}
	return next.Step(B{}, n-1)
}

// branchedRecurse picks its next step from a function value chosen behind a
// branch, then calls the variable.
//
// known-miss: f is bound to one of two different functions depending on
// flag, so the call through f is a call through a value, not a call to a
// known function; StaticCalls drops it, so the cycle branchedRecurseA and
// branchedRecurseB would otherwise complete through branchedRecurse never
// closes in the static call graph.
func branchedRecurse(flag bool, n int) int {
	var f func(bool, int) int
	if flag {
		f = branchedRecurseA
	} else {
		f = branchedRecurseB
	}
	return f(flag, n)
}

func branchedRecurseA(flag bool, n int) int {
	return branchedRecurse(flag, n-1)
}

func branchedRecurseB(flag bool, n int) int {
	return branchedRecurse(flag, n-1)
}

// Walker is satisfied by Tree. visitWalker calls it through a type-parameter
// value, and Tree.Walk calls visitWalker back.
type Walker interface{ Walk(n int) }

// visitWalker and Tree.Walk recurse into each other, but the call w.Walk is
// made on a type-parameter value.
//
// known-miss: SSA builds w.Walk as a call with no static callee, so the cycle
// never closes and stays silent.
func visitWalker[T Walker](w T, n int) {
	if n == 0 {
		return
	}
	w.Walk(n)
}

type Tree struct{}

func (t Tree) Walk(n int) { visitWalker(t, n-1) }

type Visitor interface{ Visit(v Visitor, n int) }

type Deep struct{}

// known-miss: a method value taken from an interface forwards through an
// interface call, so there is no static callee and the cycle stays silent.
func (Deep) Visit(v Visitor, n int) {
	f := v.Visit
	f(v, n-1)
}

// known-miss: a method expression on an interface type forwards through an
// interface call, so there is no static callee and the cycle stays silent.
func (Deep) VisitExpr(v Visitor, n int) {
	f := Visitor.Visit
	f(v, v, n-1)
}
