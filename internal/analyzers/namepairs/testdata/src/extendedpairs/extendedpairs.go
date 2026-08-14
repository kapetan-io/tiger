// Package fixture proves the -pairs flag extends the committed table with
// a project-owned pair (legacy=modern) on top of the built-in ones.
package fixture

// legacyPath is banned once -pairs=legacy=modern is set.
func legacyPath() string { // want `TS-N15: "legacyPath" uses the banned half of a known name pair .* rename to use "modern"`
	return ""
}

// sourcePath still uses the approved half of the built-in src/source pair,
// proving the flag extends the table instead of replacing it.
func sourcePath() string {
	return ""
}
