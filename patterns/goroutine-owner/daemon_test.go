package goroutineowner_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	goroutineowner "github.com/kapetan-io/tiger/patterns/goroutine-owner"
)

const windDown = 50 * time.Millisecond

var errFlush = errors.New("flush failed")

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
	os.Exit(m.Run())
}

func sleepWindDown() { time.Sleep(windDown) }

func flushOK() error { return nil }

func flushFails() error { return errFlush }

// TestDaemonShutdownWaits starts the daemon and shuts it down, with a flush
// that works and one that fails.
//
// Goal: Shutdown returns only after the goroutine has finished winding down,
// whichever way the flush goes.
func TestDaemonShutdownWaits(t *testing.T) {
	for _, test := range []struct {
		name    string
		flush   func() error
		wantErr error
	}{
		{name: "flush works", flush: flushOK},
		{name: "flush fails", flush: flushFails, wantErr: errFlush},
	} {
		t.Run(test.name, func(t *testing.T) {
			var daemon goroutineowner.Daemon
			daemon.Start(sleepWindDown)
			err := daemon.Shutdown(test.flush)
			require.ErrorIs(t, err, test.wantErr)
			assert.True(t, daemon.Finished())
		})
	}
}

// TestDaemonSkipsWaitReturnsEarly shuts the broken daemon down with a flush
// that fails.
//
// Goal: the broken shape returns from Shutdown while the goroutine still runs,
// where Daemon waits for it. Tiger accepts it because Shutdown does reach a Wait.
func TestDaemonSkipsWaitReturnsEarly(t *testing.T) {
	var daemon goroutineowner.DaemonSkipsWait
	daemon.Start(sleepWindDown)
	require.ErrorIs(t, daemon.Shutdown(flushFails), errFlush)
	assert.False(t, daemon.Finished())
}
