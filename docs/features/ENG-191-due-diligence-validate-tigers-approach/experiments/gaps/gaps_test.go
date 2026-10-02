package gaps_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gaps"
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

// Walk ends on a tree: three nodes, three visits.
func TestWalkEndsOnATree(t *testing.T) {
	leaf1, leaf2 := &gaps.Node{}, &gaps.Node{}
	assert.Equal(t, 3, gaps.Walk(&gaps.Node{Next: []*gaps.Node{leaf1, leaf2}}))
}

// Walk never ends on a cycle, though tiger reports nothing on it.
func TestWalkRunsForeverOnACycle(t *testing.T) {
	a, b := &gaps.Node{}, &gaps.Node{}
	a.Next = []*gaps.Node{b}
	b.Next = []*gaps.Node{a}
	assert.False(t, finishes(func() { gaps.Walk(a) }))
}

// All three game loops stop when ctx is cancelled, including the one tiger blocks.
func TestGameLoopsStopOnCancel(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(context.Context, func())
	}{
		{name: "Ticker", run: gaps.RunTicker},
		{name: "Flat", run: gaps.RunFlat},
		{name: "UntilCancelled", run: gaps.RunUntilCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			frames := 0
			assert.True(t, finishes(func() {
				test.run(ctx, func() {
					frames++
					if frames == 3 {
						cancel()
					}
				})
			}))
			assert.GreaterOrEqual(t, frames, 3)
		})
	}
}
