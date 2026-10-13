// The compliant forms: well-formed pins parse silently (they carry
// expressions, not excuses), and prose that merely
// resembles a directive is not one.
package fixture

// Pin directives are well-formed.
//
//tiger:effects mutate(r.log, r.checkpoint), io(disk)
//tiger:frame r.log, r.checkpoint
//tiger:variant len(pending)
func pinnedShapes(pending []int) int {
	drained := 0
	for range pending {
		drained++
	}
	return drained
}

// proseNotDirective mentions tiger conventions in prose. A space after the
// slashes is prose, matching Go's own //go: convention:
// tiger:bounded is withdrawn, and writing about //tiger: in running text
// with a space stays prose.
func proseNotDirective() {}
