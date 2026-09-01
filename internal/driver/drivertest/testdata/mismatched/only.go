package only

// Value is expected to fire nothing, but the harness expects a finding
// here that never comes, while the package clause above fires unexpected.
func Value() int { // want "probe: never"
	return 1
}
