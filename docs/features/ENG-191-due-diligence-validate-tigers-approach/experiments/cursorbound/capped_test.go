package cursorbound_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"cursorbound"
)

// unlimited marks a database that ignores LIMIT and never runs out of rows.
const unlimited = -1

// fakeRows stands in for a database cursor. It returns sqlLimit rows, or
// rows forever when sqlLimit is unlimited, and counts every Next call.
type fakeRows struct {
	sqlLimit  int
	sent      int
	nextCalls int
}

func (f *fakeRows) Next() bool {
	f.nextCalls++
	if f.sqlLimit != unlimited && f.sent >= f.sqlLimit {
		return false
	}
	f.sent++
	return true
}

func (f *fakeRows) Value() string {
	return fmt.Sprintf("row-%d", f.sent)
}

// TestListCappedBoundHoldsWhateverTheDatabaseDoes proves the restated limit
// bounds the loop whether the database stops first, the loop stops first,
// or the database ignores LIMIT entirely.
func TestListCappedBoundHoldsWhateverTheDatabaseDoes(t *testing.T) {
	for _, test := range []struct {
		name      string
		sqlLimit  int
		loopLimit int
		wantRows  int
		wantNext  int
	}{
		{
			name:      "DatabaseStopsFirst",
			sqlLimit:  50,
			loopLimit: 100,
			wantRows:  50,
			wantNext:  51,
		},
		{
			name:      "LimitsAgree",
			sqlLimit:  100,
			loopLimit: 100,
			wantRows:  100,
			wantNext:  100,
		},
		{
			// The loop silently drops rows the query returned. That is a
			// correctness bug for tests and review, not an unbounded loop.
			name:      "LoopStopsFirstAndTruncates",
			sqlLimit:  100,
			loopLimit: 50,
			wantRows:  50,
			wantNext:  50,
		},
		{
			name:      "DatabaseIgnoresLimit",
			sqlLimit:  unlimited,
			loopLimit: 100,
			wantRows:  100,
			wantNext:  100,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := &fakeRows{sqlLimit: test.sqlLimit}
			names := cursorbound.ListCapped(rows, test.loopLimit)
			assert.Len(t, names, test.wantRows)
			assert.Equal(t, test.wantNext, rows.nextCalls)
		})
	}
}

// TestListUncappedRunsAsLongAsTheDatabaseSends proves the uncapped loop's
// bound is whatever the database returns: a million rows means a million passes.
func TestListUncappedRunsAsLongAsTheDatabaseSends(t *testing.T) {
	const sqlLimit = 1_000_000
	rows := &fakeRows{sqlLimit: sqlLimit}
	names := cursorbound.ListUncapped(rows)
	assert.Len(t, names, sqlLimit)
	assert.Equal(t, sqlLimit+1, rows.nextCalls)
}
