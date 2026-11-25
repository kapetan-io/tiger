// A function that never calls Wait waits on nothing, even when it has no
// success exit: one that loops forever, or returns only a non-nil error, does
// not stand in for the Wait of a group started beside it or before it.
package fixture

import (
	"errors"
	"sync"
)

var errClosed = errors.New("closed")

// Forever starts a worker and has an exported method that never returns.
type Forever struct{ wg sync.WaitGroup }

// Start runs the worker.
func (f *Forever) Start() {
	f.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run loops forever and never waits.
func (f *Forever) Run() {
	for {
	}
}

// Sentinel starts a worker and has an exported method that returns only an
// error.
type Sentinel struct{ wg sync.WaitGroup }

// Start runs the worker.
func (s *Sentinel) Start() {
	s.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed reports the group closed without waiting on it.
func (s *Sentinel) Closed() error { return errClosed }

var pool sync.WaitGroup

// StartPool starts a worker on a package-level group.
func StartPool() {
	pool.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Spin loops forever and never waits.
func Spin() {
	for {
	}
}

// startThenFail calls a function that returns only an error after the start,
// and nothing waits.
func startThenFail() {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	_ = fail()
}

// startThenReturnFail returns the result of a function that returns only an
// error, and nothing waits.
func startThenReturnFail() error {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	return fail()
}

func fail() error { return errClosed }
