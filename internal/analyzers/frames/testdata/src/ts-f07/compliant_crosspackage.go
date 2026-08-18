// The compliant cross-package shape: the pin declares exactly the frame
// the helper's fact contributes, so sparse pins keep their dense
// enforcement without a finding.
package fixture

import "ts-f07helper"

// Rotate declares the helper's write as its own frame — the fact rebased
// through the parameter matches the pin exactly.
//
//tiger:frame l.Entries
func Rotate(l *helper.Ledger) { // want Rotate:`0\.Entries`
	helper.Scrub(l)
}
