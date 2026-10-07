package call7_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"call7"
)

// stuck is how long a loop must keep running before the test calls it endless.
const stuck = 300 * time.Millisecond

// finishes runs loop in a goroutine and reports whether it returned within stuck.
// A loop that never returns keeps its goroutine until the test binary exits.
func finishes(loop func()) bool {
	done := make(chan bool)
	go func() {
		loop()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(stuck):
		return false
	}
}

// A Done method that never fires passes today's rules, and the loop never stops.
func TestDrainLatchNeverStops(t *testing.T) {
	assert.False(t, finishes(func() { call7.DrainLatch(call7.Latch{}, make(chan int)) }))
}

// A channel named shutdown that nothing closes passes today's rules, and the loop never stops.
func TestWorkerNeverStops(t *testing.T) {
	assert.False(t, finishes(func() { call7.NewWorker(make(chan int)).Run() }))
}

// An empty Reset passes today's TS-M05, and the next pool user sees the last user's data.
func TestEmptyResetLeaksTheLastUser(t *testing.T) {
	leaked := false
	for range 100 {
		call7.UseWithEmptyReset("alice")
		if call7.PeekPooled() == "alice" {
			leaked = true
			break
		}
	}
	assert.True(t, leaked)
}

// Zeroing before Put never leaks the last user.
func TestZeroBeforePutNeverLeaks(t *testing.T) {
	for range 100 {
		call7.UseWithZero("bob")
		assert.NotEqual(t, "bob", call7.PeekPooled())
	}
}

// windDown is how long each daemon's goroutine takes to exit after stop.
const windDown = 50 * time.Millisecond

func sleepWindDown() { time.Sleep(windDown) }

// Add, go and Done without Wait is querator's trial bug: Shutdown returns while the goroutine runs.
func TestUnwaitedDaemonReturnsEarly(t *testing.T) {
	var d call7.UnwaitedDaemon
	d.Start(sleepWindDown)
	d.Shutdown()
	assert.False(t, d.Finished())
}

// wg.Go with Wait in Shutdown returns only after the goroutine exits.
func TestWaitedDaemonWaits(t *testing.T) {
	var d call7.WaitedDaemon
	d.Start(sleepWindDown)
	d.Shutdown()
	assert.True(t, d.Finished())
}

// wg.Go without Wait passes today's TS-C02 and has the trial bug.
func TestGoNoWaitDaemonReturnsEarly(t *testing.T) {
	var d call7.GoNoWaitDaemon
	d.Start(sleepWindDown)
	d.Shutdown()
	assert.False(t, d.Finished())
}

// ENG-160's shape: work handled after Shutdown starts makes the loop exit without
// cleanup, and Shutdown waits out its deadline.
func TestEarlyExitLoopSkipsCleanup(t *testing.T) {
	l := call7.NewEarlyExitLoop()
	started, release := make(chan struct{}), make(chan struct{})
	l.Submit(func() { close(started); <-release })
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	shutdownErr := make(chan error)
	go func() { shutdownErr <- l.Shutdown(ctx) }()
	require.Eventually(t, l.InShutdown, time.Second, time.Millisecond)
	close(release)

	require.ErrorIs(t, <-shutdownErr, context.DeadlineExceeded)
	assert.False(t, l.Cleaned())
}

// The stop-and-done shape runs cleanup even when work is handled after Shutdown starts.
func TestStopDoneLoopAlwaysCleansUp(t *testing.T) {
	l := call7.NewStopDoneLoop()
	started, release := make(chan struct{}), make(chan struct{})
	l.Submit(func() { close(started); <-release })
	<-started
	l.Submit(func() {})

	shutdownErr := make(chan error)
	go func() { shutdownErr <- l.Shutdown(context.Background()) }()
	close(release)

	require.NoError(t, <-shutdownErr)
	assert.True(t, l.Cleaned())
}

// When the caller's ctx expires the shutdown still finishes, and a retry sees it done.
func TestStopDoneLoopSurvivesAnExpiredShutdown(t *testing.T) {
	l := call7.NewStopDoneLoop()
	started, release := make(chan struct{}), make(chan struct{})
	l.Submit(func() { close(started); <-release })
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, l.Shutdown(ctx), context.DeadlineExceeded)
	close(release)

	require.NoError(t, l.Shutdown(context.Background()))
	assert.True(t, l.Cleaned())
}

// Two concurrent Shutdown calls both return only after cleanup.
func TestStopDoneLoopDoubleShutdownBothWait(t *testing.T) {
	l := call7.NewStopDoneLoop()
	started, release := make(chan struct{}), make(chan struct{})
	l.Submit(func() { close(started); <-release })
	<-started

	first := make(chan error)
	second := make(chan error)
	go func() { first <- l.Shutdown(context.Background()) }()
	go func() { second <- l.Shutdown(context.Background()) }()
	close(release)

	require.NoError(t, <-first)
	require.NoError(t, <-second)
	assert.True(t, l.Cleaned())
}
