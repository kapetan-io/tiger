// known-miss: TS-E02 only checks that a justification comment is present,
// never that it truthfully explains the discard. A vacuous or unrelated
// comment still satisfies the shape check — the reason is for a human
// reviewer to judge, not this analyzer.
package fixture

import "os"

func closeVacuousComment(f *os.File) {
	_ = f.Close() // ok
}
