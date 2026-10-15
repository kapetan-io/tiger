package clampedlimit_test

import (
	"bufio"
	"errors"
	"io"
	"math"
	"strings"
	"sync"
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

// releaseFunc ends an endless stream.
type releaseFunc func()

// finishes reports whether loop returns within stuck, without hanging the test.
// A loop that does not return is released through release, so its goroutine
// ends before finishes does.
func finishes(loop func(), release releaseFunc) bool {
	done := make(chan struct{})
	var group errgroup.Group
	group.Go(func() error {
		loop()
		close(done)
		return nil
	})
	finished := false
	select {
	case <-done:
		finished = true
	case <-time.After(stuck):
		release()
	}
	return group.Wait() == nil && finished
}

// stream is an endless stream and the function that ends it, which may run
// more than once.
type stream struct {
	rows    endlessRows
	release releaseFunc
}

// endless returns a stream that never ends until its release runs.
func endless() stream {
	stop := make(chan struct{})
	var once sync.Once
	return stream{
		rows:    endlessRows{stop: stop},
		release: func() { once.Do(func() { close(stop) }) },
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
			got, err := clampedlimit.ListPage(scanner, req)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

// TestListPageStopsWhenTheStreamEnds reads a short stream with a large limit.
//
// Goal: ListPage returns the rows that exist when the stream ends before the limit.
func TestListPageStopsWhenTheStreamEnds(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("a\nb\nc\n"))
	got, err := clampedlimit.ListPage(scanner, clampedlimit.ListRequest{Limit: 500})
	require.NoError(t, err)
	assert.Equal(t, 3, got)
}

// TestListPageHugeNeverFinishes sends a large limit that is below the huge
// maximum at an endless stream.
//
// Goal: the broken shape is clamped and still runs without end, where ListPage
// finishes with the same input.
func TestListPageHugeNeverFinishes(t *testing.T) {
	req := clampedlimit.ListRequest{Limit: math.MaxInt32}
	huge := endless()
	defer huge.release()
	assert.False(t, finishes(func() {
		_, err := clampedlimit.ListPageHuge(bufio.NewScanner(huge.rows), req)
		assert.NoError(t, err)
	}, huge.release))
	clamped := endless()
	defer clamped.release()
	require.True(t, finishes(func() {
		_, err := clampedlimit.ListPage(bufio.NewScanner(clamped.rows), req)
		assert.NoError(t, err)
	}, clamped.release))
}

// TestListPageReportsAFailedRead reads a stream that fails after one row.
//
// Goal: a read error comes back as an error and never as a short page.
func TestListPageReportsAFailedRead(t *testing.T) {
	scanner := bufio.NewScanner(io.MultiReader(strings.NewReader("a\n"), failingReader{}))
	_, err := clampedlimit.ListPage(scanner, clampedlimit.ListRequest{Limit: 10})
	require.ErrorIs(t, err, errRead)
}

var errRead = errors.New("read failed")

// failingReader fails every read.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errRead }
