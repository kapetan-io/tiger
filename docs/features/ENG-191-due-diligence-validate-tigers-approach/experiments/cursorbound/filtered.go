package cursorbound

import (
	"errors"
	"strings"
)

// pageMax is the most rows one list call returns.
const pageMax = 1_000

// scanMax is the most rows one list call reads, matched or not. It bounds
// how long a filtered list runs, the way pageMax bounds how much it returns.
const scanMax = 10_000

// A scan shorter than a page could never fill a dense page.
const _ = uint(scanMax - pageMax)

// ErrScanLimit reports that a filtered list read scanMax rows before it
// filled the page or reached the end, so the page may be missing matches.
var ErrScanLimit = errors.New("list read scanMax rows before the page filled")

// ListFilteredCountRows restates the limit and counts every row read, so rows
// the filter skips use up the page and a page comes back short.
func ListFilteredCountRows(rows Rows, namespace string, limit int) []string {
	var names []string
	for count := 0; count < limit && rows.Next(); count++ {
		name := rows.Value()
		if !strings.HasPrefix(name, namespace+"/") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// ListFilteredCountMatches counts only the rows it keeps, so the page is
// right but the loop reads every row before the page fills.
func ListFilteredCountMatches(rows Rows, namespace string, limit int) []string {
	var names []string
	for count := 0; count < limit && rows.Next(); {
		name := rows.Value()
		if !strings.HasPrefix(name, namespace+"/") {
			continue
		}
		names = append(names, name)
		count++
	}
	return names
}

// ListFilteredTwoLimits counts matches against limit and rows read against
// scanMax, and reports when scanMax stops the loop.
func ListFilteredTwoLimits(rows Rows, namespace string, limit int) ([]string, error) {
	limit = min(limit, pageMax)
	var names []string
	scanned := 0
	for ; scanned < scanMax && rows.Next(); scanned++ {
		name := rows.Value()
		if !strings.HasPrefix(name, namespace+"/") {
			continue
		}
		names = append(names, name)
		if len(names) == limit {
			return names, nil
		}
	}
	if scanned == scanMax {
		return names, ErrScanLimit
	}
	return names, nil
}
