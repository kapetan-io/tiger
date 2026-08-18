// Documented TS-V03 coverage gaps: cases the restricted predicate language
// (nil-ness, integer ranges, length relations, invariant IDs) cannot
// decide, so contracts stays silent and the obligation degrades to the
// callee's own runtime assertion (TS-A02), the spec's stated default.
package fixture

import "assert"

// flowNilOnlyKnownMiss passes p to needsPointer; p is nil only because
// nothing below ever assigns it, a fact only data-flow analysis can see.
//
// known-miss: contracts proves a violation only from a literal nil
// argument; a nil established through flow analysis is invisible to the
// predicate language and degrades to needsPointer's runtime assertion.
func flowNilOnlyKnownMiss() int {
	var p *int
	return needsPointer(p)
}

// withdraw requires the invariant inv.NoOverdraft. Its predicate is a bare
// invariant ID, never a comparison the analyzer can evaluate, so it never
// proves a call site false.
//
//tiger:requires inv.NoOverdraft
func withdraw(amount int) int {
	return amount
}

// callWithdrawKnownMiss passes a negative amount, which would violate the
// stated invariant if anything could check it here.
//
// known-miss: a bare invariant-ID predicate is never provable by constant
// reasoning — contracts always treats it as silent, degrading to the
// runtime assertion the invariant names.
func callWithdrawKnownMiss() int {
	return withdraw(-1)
}

// crossPackageKnownMiss calls assert.Positive with 0, which fails its own
// requires n > 0 — but that pin lives in a different package, and
// contracts only resolves same-package call sites.
//
// known-miss: cross-package requires enforcement needs the facts plumbing
// a later wave builds; today the call degrades to Positive's own runtime
// assertion.
func crossPackageKnownMiss() int {
	return assert.Positive(0)
}
