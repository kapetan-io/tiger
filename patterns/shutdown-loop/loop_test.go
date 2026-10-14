package shutdownloop_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shutdownloop "github.com/kapetan-io/tiger/patterns/shutdown-loop"
)

func expiredContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// TestCallReturnsResult runs work on the loop.
//
// Goal: Call returns what the work function returned.
func TestCallReturnsResult(t *testing.T) {
	loop := shutdownloop.NewLoop()
	result, err := loop.Call(context.Background(), func() int { return 7 })
	require.NoError(t, err)
	assert.Equal(t, 7, result)
	require.NoError(t, loop.Shutdown(context.Background()))
}

// TestShutdownFromInsideWorkStillCleansUp starts Shutdown from inside a request
// with an expired context, the ENG-160 sequence.
//
// Goal: the loop still reaches cleanup and a later Shutdown sees it finished.
func TestShutdownFromInsideWorkStillCleansUp(t *testing.T) {
	loop := shutdownloop.NewLoop()
	var inside error
	_, err := loop.Call(context.Background(), func() int {
		inside = loop.Shutdown(expiredContext())
		return 0
	})
	require.NoError(t, err)
	require.ErrorIs(t, inside, context.Canceled)

	require.NoError(t, loop.Shutdown(context.Background()))
	assert.True(t, loop.Cleaned())
}

// TestShutdownContextBoundsOnlyTheWait calls Shutdown with an expired context.
//
// Goal: Shutdown returns the context error, the loop still stops, and a retry returns nil.
func TestShutdownContextBoundsOnlyTheWait(t *testing.T) {
	loop := shutdownloop.NewLoop()
	require.ErrorIs(t, loop.Shutdown(expiredContext()), context.Canceled)

	require.NoError(t, loop.Shutdown(context.Background()))
	assert.True(t, loop.Cleaned())
}

// TestCallAfterShutdownReturnsClosed calls the loop after it finished, when the
// buffered requests channel still accepts the request.
//
// Goal: the waiter on the reply also selects on done, so Call returns ErrClosed.
func TestCallAfterShutdownReturnsClosed(t *testing.T) {
	loop := shutdownloop.NewLoop()
	require.NoError(t, loop.Shutdown(context.Background()))

	_, err := loop.Call(context.Background(), func() int { return 1 })
	assert.ErrorIs(t, err, shutdownloop.ErrClosed)
}
