// The TS-N08 failure mode: a plain bool parameter.
package fixture

// save persists state and takes a plain bool for whether to sync
// immediately.
func save(immediate bool) { // want `TS-N08: plain bool parameter`
	_ = immediate
}

// configure takes a bool among other parameters.
func configure(name string, verbose bool) { // want `TS-N08: plain bool parameter`
	_, _ = name, verbose
}


// Saver is an interface whose method takes a plain bool parameter.
type Saver interface {
	// Save takes a plain bool parameter.
	Save(immediate bool) // want `TS-N08: plain bool parameter`
}
