// A package-qualified boolean call fires whatever directive it carries.
package fixture

import "strings"

func pkgCallBatchedStillFires(s string) int {
	count := 0
	//tiger:batched a package function call states no limit
	for strings.Contains(s, "x") { // want `TS-S02: tiger can't tell how many times this loop runs`
		count++
	}
	return count
}
