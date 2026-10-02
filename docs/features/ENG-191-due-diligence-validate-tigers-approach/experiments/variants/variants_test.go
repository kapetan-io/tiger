package variants_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"variants"
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

// Tiger verifies variant i, and the countdown ends.
func TestCountdownEnds(t *testing.T) {
	assert.Equal(t, 5, variants.Countdown(5))
}

// Tiger verifies variant n - i, and the loop ends, though TS-S02 still blocks it.
func TestEveryOtherEnds(t *testing.T) {
	assert.Equal(t, 0+2+4+6+8, variants.EveryOther(10))
}

// Tiger verifies variant high - low, and the loop ends, though TS-S02 still blocks it.
func TestReverseEnds(t *testing.T) {
	s := []int{1, 2, 3, 4, 5}
	variants.Reverse(s)
	assert.Equal(t, []int{5, 4, 3, 2, 1}, s)
}

// Tiger verifies variant len(stack), and the stack empties.
func TestPopAllEnds(t *testing.T) {
	assert.Equal(t, 3, variants.PopAll([]int{1, 2, 3}))
}

// Tiger rejects variant len(q.Items) because the condition reads a field, but the loop ends.
func TestDrainQueueEnds(t *testing.T) {
	assert.Equal(t, 3, variants.DrainQueue(&variants.Queue{Items: []int{1, 2, 3}}))
}

// Tiger rejects this variant, and the loop really does not end: i walks away from n.
func TestWrongWayRunsForever(t *testing.T) {
	assert.False(t, finishes(func() { variants.WrongWay(10) }))
}

// Tiger rejects this variant, but the loop ends: 300 needs two 7-bit groups.
func TestVarintEnds(t *testing.T) {
	assert.Equal(t, 2, variants.Varint(300))
}

// Tiger rejects this variant, but the loop ends: 10 bytes in chunks of 4 is 3 chunks.
func TestChunksEnds(t *testing.T) {
	assert.Equal(t, 3, variants.Chunks(make([]byte, 10), 4))
}
