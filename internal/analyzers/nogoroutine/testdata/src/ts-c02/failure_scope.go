// Starts the call 7 reviewers found a check could miss: a method value hides
// the call, and a promoted method hides the group.
package fixture

import "sync"

func methodValue(f func()) {
	var wg sync.WaitGroup
	start := wg.Go // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	start(f)
}

// Embedded promotes Go from the embedded group, and nothing waits on it.
type Embedded struct {
	sync.WaitGroup
}

// Start runs f on the embedded group.
func (e *Embedded) Start(f func()) {
	e.Go(f) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}
