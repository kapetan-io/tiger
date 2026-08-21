// A package-qualified boolean call is not the cursor shape: the waiver
// recognizes a method call on a cursor value (it.Valid(), rows.Next()),
// and a package function has no cursor to advance, so the directive does
// not attach a waiver here.
package fixture

import "strings"

func pkgCallBatchedStillFires(s string) int {
	count := 0
	//tiger:batched not a cursor; a package function scans nothing forward
	for strings.Contains(s, "x") { // want `TS-S02: this loop's bound cannot be derived`
		count++
	}
	return count
}
