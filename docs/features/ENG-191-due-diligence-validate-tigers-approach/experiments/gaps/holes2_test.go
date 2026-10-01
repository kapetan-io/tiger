package gaps_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"gaps"
)

// Counter loops tiger accepts that run forever.
func TestCounterLoopsTigerAcceptsRunForever(t *testing.T) {
	for _, test := range []struct {
		name string
		loop func()
	}{
		{name: "OrCounter", loop: func() { gaps.OrCounter(func() bool { return false }) }},
		{name: "CounterReset", loop: gaps.CounterReset},
		{name: "FieldNamedLikeCounter", loop: func() { gaps.FieldNamedLikeCounter(gaps.Register()) }},
		{name: "StepsPastLimit", loop: gaps.StepsPastLimit},
		{name: "ByteWraps", loop: gaps.ByteWraps},
		{name: "SpinCapped", loop: func() { gaps.SpinCapped(func() bool { return false }) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.False(t, finishes(test.loop))
		})
	}
}

// The capped worklist ends on a cycle but returns a partial walk with no error.
func TestWalkCappedTruncatesSilentlyOnACycle(t *testing.T) {
	a, b := &gaps.Node{}, &gaps.Node{}
	a.Next = []*gaps.Node{b}
	b.Next = []*gaps.Node{a}
	assert.Equal(t, 101, gaps.WalkCapped(a))
}

// Ranging over the worklist visits only the root, never the nodes it appends.
func TestWalkRangeSkipsAppendedNodes(t *testing.T) {
	leaf1, leaf2 := &gaps.Node{}, &gaps.Node{}
	assert.Equal(t, 1, gaps.WalkRange(&gaps.Node{Next: []*gaps.Node{leaf1, leaf2}}))
}

// A ctx.Err() loop on a context nobody can cancel never ends.
func TestRunUntilCancelledNeverEndsOnBackground(t *testing.T) {
	assert.False(t, finishes(func() { gaps.RunUntilCancelled(context.Background(), func() {}) }))
}
