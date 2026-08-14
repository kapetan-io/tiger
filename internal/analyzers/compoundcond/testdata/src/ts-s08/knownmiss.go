// known-miss: TS-S08 only searches the closed set's own package (kind,
// here) for constants of the switched type. These constants are declared in
// this package instead, so the same-package search finds nothing and treats
// the switch as open even though every kind.Status value is covered below.
package fixture

import "kind"

const (
	KindActive kind.Status = iota
	KindInactive
)

// switchCrossPackageConstants switches over a set that is closed in
// practice but open by TS-S08's per-package search, so no default is
// required here.
func switchCrossPackageConstants(status kind.Status) string {
	switch status {
	case KindActive:
		return "active"
	case KindInactive:
		return "inactive"
	}
	return "unknown"
}
