// More TS-C02 compliant shapes: groups in package variables, embedded groups,
// generic owners, a Wait inside a deferred closure, and a Wait on both the
// success path and the expired-context path.
package fixture

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

var sweepGroup sync.WaitGroup

// Sweeping starts a goroutine on a package-level group.
func Sweeping() { sweepGroup.Go(work) }

// StopSweeping waits through a helper.
func StopSweeping() { reapAll() }

func reapAll() { sweepGroup.Wait() }

// EmbeddedWaited embeds the WaitGroup and waits on it.
type EmbeddedWaited struct {
	sync.WaitGroup
}

// Start runs the worker.
func (e *EmbeddedWaited) Start() { e.Go(work) }

// Close waits for the worker.
func (e *EmbeddedWaited) Close() { e.Wait() }

func deferredClosureWait() {
	var wg sync.WaitGroup
	defer func() { wg.Wait() }()
	wg.Go(work)
}

// Gang is a generic owner that waits in an exported method.
type Gang[T any] struct {
	wg    sync.WaitGroup
	items chan T
}

// Start runs the worker.
func (g *Gang[T]) Start() { g.wg.Go(work) }

// Close waits for the worker.
func (g *Gang[T]) Close() { g.wg.Wait() }

// Both waits on the success path and, separately, when ctx expires.
type Both struct {
	wg   sync.WaitGroup
	done chan struct{}
}

// Start runs the worker.
func (b *Both) Start() { b.wg.Go(work) }

// Shutdown waits on either path.
func (b *Both) Shutdown(ctx context.Context) error {
	select {
	case <-b.done:
		b.wg.Wait()
		return nil
	case <-ctx.Done():
		b.wg.Wait()
		return ctx.Err()
	}
}

func tryGoWaited() error {
	var g errgroup.Group
	if !g.TryGo(func() error { return nil }) {
		return nil
	}
	return g.Wait()
}

// Aliased starts the goroutine through a local pointer to its own field.
type Aliased struct {
	wg sync.WaitGroup
}

// Start runs the worker through an alias of the field.
func (a *Aliased) Start() {
	group := &a.wg
	group.Go(work)
}

// Close waits for the worker.
func (a *Aliased) Close() { a.wg.Wait() }
