package clampedlimit_test

import (
	"bufio"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/kapetan-io/tiger/patterns/clamped-limit"
)

// stuck is how long a loop may run before the test calls it endless.
const stuck = 200 * time.Millisecond

// endlessRows is a stream with more rows than any page; it ends only when
// stop closes, so a loop that outlives its test does not keep a core busy.
type endlessRows struct {
	stop chan struct{}
}

func (rows endlessRows) Read(buffer []byte) (int, error) {
	select {
	case <-rows.stop:
		return 0, io.EOF
	default:
	}
	for i := range buffer {
		buffer[i] = '\n'
	}
	return len(buffer), nil
}

// finishes reports whether loop returns within stuck, without hanging the test.
func finishes(loop func()) bool {
	done := make(chan struct{})
	var group errgroup.Group
	group.Go(func() error {
		loop()
		close(done)
		return nil
	})
	select {
	case <-done:
		return true
	case <-time.After(stuck):
		return false
	}
}

// TestListPageClampsTheLimit sends huge, zero and negative limits at a stream
// with more rows than any page.
//
// Goal: ListPage returns at most the page maximum, and at least one row, for any limit.
func TestListPageClampsTheLimit(t *testing.T) {
	for _, test := range []struct {
		name  string
		limit int32
		want  int
	}{
		{name: "huge", limit: math.MaxInt32, want: 1_000},
		{name: "justOverMax", limit: 1_001, want: 1_000},
		{name: "ordinary", limit: 25, want: 25},
		{name: "zero", limit: 0, want: 1},
		{name: "negative", limit: -5, want: 1},
		{name: "minimum", limit: math.MinInt32, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			stop := make(chan struct{})
			defer close(stop)
			scanner := bufio.NewScanner(endlessRows{stop: stop})
			req := clampedlimit.ListRequest{Limit: test.limit}
			assert.Equal(t, test.want, clampedlimit.ListPage(scanner, req))
		})
	}
}

// TestListPageStopsWhenTheStreamEnds reads a short stream with a large limit.
//
// Goal: ListPage returns the rows that exist when the stream ends before the limit.
func TestListPageStopsWhenTheStreamEnds(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("a\nb\nc\n"))
	assert.Equal(t, 3, clampedlimit.ListPage(scanner, clampedlimit.ListRequest{Limit: 500}))
}

// TestListPageHugeNeverFinishes sends a large limit that is below the huge
// maximum at an endless stream.
//
// Goal: the broken shape is clamped and still runs without end, where ListPage
// finishes with the same input.
func TestListPageHugeNeverFinishes(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	req := clampedlimit.ListRequest{Limit: math.MaxInt32}
	assert.False(t, finishes(func() {
		clampedlimit.ListPageHuge(bufio.NewScanner(endlessRows{stop: stop}), req)
	}))
	require.True(t, finishes(func() {
		clampedlimit.ListPage(bufio.NewScanner(endlessRows{stop: stop}), req)
	}))
}
