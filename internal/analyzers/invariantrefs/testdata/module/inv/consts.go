// Package inv declares the invariant IDs this corpus exercises.
package inv

// ID names one invariant.
type ID string

const (
	// TestOnly is asserted only from a _test.go file — zero production
	// sites, so TS-A07 fires (failure case).
	TestOnly ID = "test-only" // want `TS-A07: invariant inv\.TestOnly is declared but no function outside _test\.go files asserts it`

	// NeverReferenced is a const of the invariant type that no assert
	// call anywhere names — the type is still recognized because
	// production.go passes another TestOnly-typed... no, another const of
	// this type to assert.Invariant, so this const's zero references is
	// exactly the "misfiled or under-defended" case TS-A07 exists for
	// (failure case).
	NeverReferenced ID = "never-referenced" // want `TS-A07: invariant inv\.NeverReferenced is declared but no function outside _test\.go files asserts it`

	// AssertedOnce has exactly one production assertion site — TS-A07
	// requires one, not two (compliant case).
	AssertedOnce ID = "asserted-once"

	// AssertedInClosure is asserted from a closure folded into its
	// enclosing production function (compliant case).
	AssertedInClosure ID = "asserted-in-closure"

	// AssertedInMethod is asserted from a method body (compliant case).
	AssertedInMethod ID = "asserted-in-method"
)
