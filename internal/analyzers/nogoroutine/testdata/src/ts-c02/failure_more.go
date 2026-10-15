// More TS-C02 failure modes: a Wait that exists but is not the owning type's
// to call, a Wait on a different path than the success path, a promoted Go
// with no Wait, and a bare go nested inside a group's own function.
package fixture

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// EmbeddedNoWait embeds the WaitGroup, so e.Go starts the goroutine and
// nothing ever calls e.Wait.
type EmbeddedNoWait struct {
	sync.WaitGroup
}

// Start runs the worker.
func (e *EmbeddedNoWait) Start() {
	e.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Close returns without waiting.
func (e *EmbeddedNoWait) Close() {}

var sweepers sync.WaitGroup

// Sweep starts a goroutine on a package-level group.
func Sweep() {
	sweepers.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// reapSweepers waits, but nothing exported reaches it.
func reapSweepers() { sweepers.Wait() }

// Crew owns the group.
type Crew struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (c *Crew) Start() {
	c.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Boss waits on a group that belongs to Crew, not to Boss.
type Boss struct {
	crew *Crew
}

// Close waits on another type's group.
func (b *Boss) Close() { b.crew.wg.Wait() }

// Rig owns the group.
type Rig struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (r *Rig) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// CloseRig is an exported function, not a method of Rig.
func CloseRig(r *Rig) { r.wg.Wait() }

func joinHelper(wg *sync.WaitGroup) { wg.Wait() }

// localWaitInHelper hands a local group to another function to wait on; the
// Wait is not in the function that holds the group.
func localWaitInHelper() {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	joinHelper(&wg)
}

// localExpiredWait waits for a local group only after ctx has expired.
func localExpiredWait(ctx context.Context, done chan struct{}) error {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		wg.Wait()
		return ctx.Err()
	}
}

// nestedBare starts a bare goroutine inside a group's function.
func nestedBare() error {
	var g errgroup.Group
	g.Go(func() error {
		go work() // want `TS-C02: this go statement starts a goroutine nobody owns`
		return nil
	})
	return g.Wait()
}

// Slack is a generic type whose group nothing waits on.
type Slack[T any] struct {
	wg    sync.WaitGroup
	items chan T
}

// Start runs the worker.
func (s *Slack[T]) Start() {
	s.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Group waits on the field of a different instance type through an alias
// of the owner's pointer, then never waits for the one it started.
type Tide struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (t *Tide) Start() {
	t.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Close waits on a copy-free local group, not the field.
func (t *Tide) Close() {
	var other sync.WaitGroup
	other.Wait()
}
