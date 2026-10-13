// The compliant forms: well-formed pins and intent declarations parse
// silently (they carry expressions, not excuses), and prose that merely
// resembles a directive is not one.
package fixture

// Pin directives and the openenum intent directive are well-formed.
//
//tiger:effects mutate(r.log, r.checkpoint), io(disk)
//tiger:frame r.log, r.checkpoint
//tiger:variant len(pending)
//tiger:openenum
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
