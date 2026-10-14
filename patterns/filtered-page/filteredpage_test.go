package filteredpage_test

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/patterns/filtered-page"
)

// shape sizes a fake table: the last matching of total rows are in namespace
// "team", the rest in "other".
type shape struct {
	total    int
	matching int
}

// scan returns a scanner over the rows of shape.
func scan(size shape) *bufio.Scanner {
	var rows strings.Builder
	for row := 1; row <= size.total; row++ {
		namespace := "other"
		if row > size.total-size.matching {
			namespace = "team"
		}
		fmt.Fprintf(&rows, "%s/row-%d\n", namespace, row)
	}
	return bufio.NewScanner(strings.NewReader(rows.String()))
}

// following returns the first row the list did not read, or "" at the end of
// the table.
func following(rows *bufio.Scanner) string {
	if !rows.Scan() {
		return ""
	}
	return rows.Text()
}

// TestListCountingRowsReturnsEmptyPage lists 10 rows of a table holding 5
// matches after 95 other rows.
//
// Goal: counting rows read returns an empty page while matches exist, so a
// client paging through the list stops early.
func TestListCountingRowsReturnsEmptyPage(t *testing.T) {
	rows := scan(shape{total: 100, matching: 5})
	names := filteredpage.ListCountingRows(rows, "team", 10)
	assert.Empty(t, names)
	assert.Equal(t, "other/row-11", following(rows))
}

// TestList lists a filtered table in dense, sparse and out of range cases.
//
// Goal: the page fills with kept rows, a sparse table returns ErrScanLimit,
// and a limit of zero or less returns nothing.
func TestList(t *testing.T) {
	for _, test := range []struct {
		name      string
		size      shape
		limit     int
		wantNames int
		wantNext  string
		wantErr   error
	}{
		{
			name:      "DenseTableFillsPage",
			size:      shape{total: 100, matching: 50},
			limit:     10,
			wantNames: 10,
			wantNext:  "team/row-61",
		},
		{
			name:      "MatchesOnlyAtEndOfTable",
			size:      shape{total: 100, matching: 5},
			limit:     10,
			wantNames: 5,
			wantNext:  "",
		},
		{
			name:      "SparseTableHitsScanLimit",
			size:      shape{total: 20_000, matching: 1},
			limit:     10,
			wantNames: 0,
			wantNext:  "other/row-10002",
			wantErr:   filteredpage.ErrScanLimit,
		},
		{
			name:      "TableOfExactlyScanMaxRowsEndsCleanly",
			size:      shape{total: 10_000, matching: 1},
			limit:     10,
			wantNames: 1,
			wantNext:  "",
		},
		{
			name:      "LimitAbovePageMaxIsClamped",
			size:      shape{total: 5_000, matching: 2_000},
			limit:     5_000,
			wantNames: 1_000,
			wantNext:  "team/row-4001",
		},
		{
			name:      "ZeroLimitReadsNothing",
			size:      shape{total: 100, matching: 50},
			limit:     0,
			wantNames: 0,
			wantNext:  "other/row-1",
		},
		{
			name:      "NegativeLimitReadsNothing",
			size:      shape{total: 100, matching: 50},
			limit:     -1,
			wantNames: 0,
			wantNext:  "other/row-1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := scan(test.size)
			names, err := filteredpage.List(rows, "team", test.limit)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Len(t, names, test.wantNames)
			assert.Equal(t, test.wantNext, following(rows))
		})
	}
}

var errRead = errors.New("read failed")

// failingRows fails every read.
type failingRows struct{}

func (failingRows) Read([]byte) (int, error) { return 0, errRead }

// TestListReportsAFailedRead lists a table whose read fails.
//
// Goal: a read error comes back as an error, never as a short page.
func TestListReportsAFailedRead(t *testing.T) {
	_, err := filteredpage.List(bufio.NewScanner(failingRows{}), "team", 10)
	require.ErrorIs(t, err, errRead)
}
