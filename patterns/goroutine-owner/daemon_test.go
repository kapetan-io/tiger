package goroutineowner_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	goroutineowner "github.com/kapetan-io/tiger/patterns/goroutine-owner"
)

const windDown = 50 * time.Millisecond

func sleepWindDown() { time.Sleep(windDown) }

// TestDaemonShutdownWaits starts the daemon and shuts it down.
//
// Goal: Shutdown returns only after the goroutine has finished winding down.
func TestDaemonShutdownWaits(t *testing.T) {
	var daemon goroutineowner.Daemon
	daemon.Start(sleepWindDown)
	daemon.Shutdown()
	assert.True(t, daemon.Finished())
}

// TestDaemonNoWaitReturnsEarly starts the no-wait daemon and shuts it down.
//
// Goal: the broken shape returns from Shutdown while the goroutine still runs,
// where Daemon waits for it.
func TestDaemonNoWaitReturnsEarly(t *testing.T) {
	var daemon goroutineowner.DaemonNoWait
	daemon.Start(sleepWindDown)
	daemon.Shutdown()
	assert.False(t, daemon.Finished())
}
