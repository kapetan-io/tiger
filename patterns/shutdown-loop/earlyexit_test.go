package shutdownloop_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shutdownloop "github.com/kapetan-io/tiger/patterns/shutdown-loop"
)

// TestEarlyExitLoopSkipsCleanup starts Shutdown from inside a request, so the
// request case sees the flag and returns.
//
// Goal: the broken shape reports a clean Shutdown while Cleaned is false.
func TestEarlyExitLoopSkipsCleanup(t *testing.T) {
	loop := shutdownloop.NewEarlyExitLoop()
	var inside error
	_, err := loop.Call(context.Background(), func() int {
		inside = loop.Shutdown(expiredContext())
		return 0
	})
	require.NoError(t, err)
	require.ErrorIs(t, inside, context.Canceled)

	require.NoError(t, loop.Shutdown(context.Background()))
	assert.False(t, loop.Cleaned())
}
