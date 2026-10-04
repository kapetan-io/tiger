package cursorbound_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cursorbound"
)

// fakeTable stands in for a store cursor over total rows. The last matching
// rows are in namespace "team", the rest in "other", and every Next call is
// counted.
type fakeTable struct {
	total     int
	matching  int
	sent      int
	nextCalls int
}

func (f *fakeTable) Next() bool {
	f.nextCalls++
	if f.sent >= f.total {
		return false
	}
	f.sent++
	return true
}

func (f *fakeTable) Value() string {
	if f.sent > f.total-f.matching {
		return fmt.Sprintf("team/row-%d", f.sent)
	}
	return fmt.Sprintf("other/row-%d", f.sent)
}

// TestListFilteredCountRowsReturnsAShortPage proves counting every row read
// returns an empty page while five matches exist, so a client paging through
// the list stops early.
func TestListFilteredCountRowsReturnsAShortPage(t *testing.T) {
	table := &fakeTable{total: 100, matching: 5}
	names := cursorbound.ListFilteredCountRows(table, "team", 10)
	assert.Empty(t, names)
	assert.Equal(t, 10, table.nextCalls)
}

// TestListFilteredCountMatchesReadsTheWholeTable proves counting only matches
// returns the right page but reads every row to find one match.
func TestListFilteredCountMatchesReadsTheWholeTable(t *testing.T) {
	const total = 1_000_000
	table := &fakeTable{total: total, matching: 1}
	names := cursorbound.ListFilteredCountMatches(table, "team", 10)
	assert.Equal(t, []string{"team/row-1000000"}, names)
	assert.Equal(t, total+1, table.nextCalls)
}

// TestListFilteredTwoLimits proves the page counts matches, and the scan
// limit stops a sparse list and reports it instead of returning a short page
// as if it were complete.
func TestListFilteredTwoLimits(t *testing.T) {
	for _, test := range []struct {
		name      string
		total     int
		matching  int
		wantNames int
		wantNext  int
		wantErr   error
	}{
		{
			name:      "PageFills",
			total:     100,
			matching:  50,
			wantNames: 10,
			wantNext:  60,
		},
		{
			name:      "TableEndsFirst",
			total:     100,
			matching:  5,
			wantNames: 5,
			wantNext:  101,
		},
		{
			name:      "ScanLimitStopsASparseList",
			total:     1_000_000,
			matching:  1,
			wantNames: 0,
			wantNext:  10_000,
			wantErr:   cursorbound.ErrScanLimit,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			table := &fakeTable{total: test.total, matching: test.matching}
			names, err := cursorbound.ListFilteredTwoLimits(table, "team", 10)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Len(t, names, test.wantNames)
			assert.Equal(t, test.wantNext, table.nextCalls)
		})
	}
}
