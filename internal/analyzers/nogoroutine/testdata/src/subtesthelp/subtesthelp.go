// Package subtesthelp is a test helper in another package, which TS-C02 does
// not follow when it looks for t.Parallel.
package subtesthelp

import "testing"

func Parallel(t *testing.T) { t.Parallel() }
