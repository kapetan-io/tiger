package gaps

import (
	"context"
	"time"
)

const frameInterval = 16 * time.Millisecond

// RunTicker draws a frame per tick until ctx ends. Tiger accepts it.
func RunTicker(ctx context.Context, frame func()) {
	tick := time.NewTicker(frameInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			frame()
		}
	}
}

// RunFlat draws frames as fast as possible until ctx ends. Tiger accepts it.
func RunFlat(ctx context.Context, frame func()) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			frame()
		}
	}
}

// RunUntilCancelled has the same exit as RunFlat, yet TS-S02 and TS-V01 both
// block it because ctx.Err() == nil is not a recognized condition.
func RunUntilCancelled(ctx context.Context, frame func()) {
	for ctx.Err() == nil {
		frame()
	}
}
