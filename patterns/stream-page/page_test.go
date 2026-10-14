package streampage_test

import (
	"bufio"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	streampage "github.com/kapetan-io/tiger/patterns/stream-page"
)

// endless marks a table that never runs out of rows.
const endless = -1

// fakeTable stands in for a sorted database table of rows named row-0001,
// row-0002, and so on. It sends rows past the `after` key, ignoring the
// requested limit as a misbehaving database would, one row per Read so sent
// counts what the scanner pulled.
type fakeTable struct {
	total int
	first int
	sent  int
}

func (f *fakeTable) Read(buffer []byte) (int, error) {
	row := f.first + f.sent
	if f.total != endless && row > f.total {
		return 0, io.EOF
	}
	f.sent++
	return copy(buffer, fmt.Sprintf("row-%04d\n", row)), nil
}

// open returns an Opener over a table of total rows and records the stream it
// opened last.
func open(t *testing.T, total int, last *fakeTable) streampage.Opener {
	return func(after string, _ int) *bufio.Scanner {
		first := 1
		if after != "" {
			_, err := fmt.Sscanf(after, "row-%d", &first)
			require.NoError(t, err)
			first++
		}
		*last = fakeTable{total: total, first: first}
		return bufio.NewScanner(last)
	}
}

// TestListPageEndsAtTheLimitOnAnEndlessTable pages a table that ignores the
// requested limit and never ends.
//
// Goal: the page holds the clamped number of rows and the loop reads no more
// lines than that, whatever the table does.
func TestListPageEndsAtTheLimitOnAnEndlessTable(t *testing.T) {
	for _, test := range []struct {
		name      string
		requested int
		wantRows  int
	}{
		{name: "OrdinaryLimit", requested: 250, wantRows: 250},
		{name: "AbovePageMax", requested: 5_000, wantRows: 1_000},
		{name: "Zero", requested: 0, wantRows: 1},
		{name: "Negative", requested: -5, wantRows: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			last := &fakeTable{}
			page := streampage.ListPage(open(t, endless, last), "", test.requested)
			assert.Len(t, page.Names, test.wantRows)
			assert.Equal(t, test.wantRows, last.sent)
		})
	}
}

// TestListPageWalksTheWholeTable follows After from page to page.
//
// Goal: a client that passes each page's After to the next call sees every row
// once, in order, and gets an empty After on the last page.
func TestListPageWalksTheWholeTable(t *testing.T) {
	const total = 25
	var seen []string
	after := ""
	for range 4 {
		page := streampage.ListPage(open(t, total, &fakeTable{}), after, 10)
		seen = append(seen, page.Names...)
		if page.After == "" {
			break
		}
		after = page.After
	}
	require.Len(t, seen, total)
	assert.Equal(t, "row-0001", seen[0])
	assert.Equal(t, "row-0025", seen[total-1])
}

// TestListFromStartRepeatsThePageForever pages with the broken shape.
//
// Goal: the second call returns the same rows as the first, where ListPage
// returns the rows after them.
func TestListFromStartRepeatsThePageForever(t *testing.T) {
	first := streampage.ListFromStart(open(t, 25, &fakeTable{}), 10)
	second := streampage.ListFromStart(open(t, 25, &fakeTable{}), 10)
	assert.Equal(t, first, second)

	page1 := streampage.ListPage(open(t, 25, &fakeTable{}), "", 10)
	page2 := streampage.ListPage(open(t, 25, &fakeTable{}), page1.After, 10)
	assert.NotEqual(t, page1.Names, page2.Names)
}
