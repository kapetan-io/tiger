package loopholes

import (
	"context"
	"time"
)

const frameInterval = 16 * time.Millisecond


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



func RunUntilCancelled(ctx context.Context, frame func()) {
	for ctx.Err() == nil {
		frame()
	}
}
