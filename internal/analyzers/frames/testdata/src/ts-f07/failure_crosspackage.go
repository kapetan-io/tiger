// The TS-F02-style cross-package failure mode for frames: the write outside
// the pinned frame is introduced in a different package, and arrives here
// through the helper's exported FrameFact rebased onto this function's own
// parameter.
package fixture

import "ts-f07helper"

// Purge claims an empty frame but hands its parameter to the helper, whose
// fact says it writes l.Entries — the violation surfaces at this pin, not
// in the helper's package.
//
// want +1 `TS-F07: computed frame writes l\.Entries, introduced at .*, outside the pinned frame`
//tiger:frame none
func Purge(l *helper.Ledger) { // want Purge:`^$`
	helper.Scrub(l)
}
