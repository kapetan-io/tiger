package annotations_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"annotations"
)

// stuck is how long a loop must keep running before the test calls it endless.
const stuck = 300 * time.Millisecond

// fakeRows returns total rows, or rows forever when endless is set.
type fakeRows struct {
	total   int
	endless bool
	sent    int
}

func (f *fakeRows) Next() bool {
	if !f.endless && f.sent >= f.total {
		return false
	}
	f.sent++
	return true
}

func (f *fakeRows) Value() string { return "row" }

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

// Honest variant: tiger reports nothing, and the loop ends.
func TestDrainEnds(t *testing.T) {
	assert.Equal(t, 3, annotations.Drain([]string{"a", "b", "c"}))
}

// Lying variant: tiger blocks it, and the loop really can run forever.
func TestDrainRefillCanRunForever(t *testing.T) {
	assert.False(t, finishes(func() {
		annotations.DrainRefill([]string{"a"}, func() []string { return []string{"again"} })
	}))
}

// Lying variant: tiger blocks it, and the loop really can run forever.
func TestDrainSkipCanRunForever(t *testing.T) {
	assert.False(t, finishes(func() {
		annotations.DrainSkip([]string{"a"}, func() bool { return true })
	}))
}

// Honest batched: tiger lets it pass TS-S02, and it ends when the query ends.
func TestListEndsWhenRowsEnd(t *testing.T) {
	assert.Len(t, annotations.List(&fakeRows{total: 50}), 50)
}

// Lying batched: tiger gives it the same findings as List, yet it never ends.
func TestCountEndlessRunsForever(t *testing.T) {
	assert.False(t, finishes(func() {
		annotations.CountEndless(&fakeRows{endless: true})
	}))
}

// Restated limit: no annotation, tiger reports nothing, and it ends even on endless rows.
func TestListRestatedEndsOnEndlessRows(t *testing.T) {
	rows := &fakeRows{endless: true}
	assert.Len(t, annotations.ListRestated(rows, 100), 100)
	assert.Equal(t, 100, rows.sent)
}
