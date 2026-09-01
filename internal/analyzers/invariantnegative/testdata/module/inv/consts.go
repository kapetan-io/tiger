// Package inv declares the invariant IDs this corpus exercises.
package inv

// ID names one invariant.
type ID string

const (
	// NoTest is asserted in production but never violated by any test —
	// TS-A09 fires (failure case).
	NoTest ID = "no-test" // want `TS-A09: no test violates invariant inv\.NoTest`

	// ViolatedOutsideTest has an assert.Violates call, but it sits in a
	// production function, not a _test.go file — TS-A09 still fires,
	// because the negative-space proof must live where a test runner
	// executes it (failure case).
	ViolatedOutsideTest ID = "violated-outside-test" // want `TS-A09: no test violates invariant inv\.ViolatedOutsideTest`

	// ViolatedInExternalTest is violated by a test in an external test
	// package (foo_test) — compliant.
	ViolatedInExternalTest ID = "violated-in-external-test"

	// ViolatedInInternalTest is violated by a test in an in-package
	// _test.go file — compliant.
	ViolatedInInternalTest ID = "violated-in-internal-test"
)
