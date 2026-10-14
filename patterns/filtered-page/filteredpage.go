// Package filteredpage lists one row per line of a scanner, the way a store
// cursor yields rows. It shows a filtered list loop that fills its page and
// reports when it gives up, beside one that returns a short page silently.
package filteredpage

import (
	"bufio"
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

// ListCountingRows restates the limit and counts every row read, so rows the
// filter skips use up the page and a page comes back short or empty.
func ListCountingRows(rows *bufio.Scanner, namespace string, limit int) []string {
	var names []string
	for count := 0; count < limit && rows.Scan(); count++ {
		name := rows.Text()
		if !strings.HasPrefix(name, namespace+"/") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// List counts kept rows against limit and rows read against scanMax, and
// reports when scanMax stops the loop.
func List(rows *bufio.Scanner, namespace string, limit int) ([]string, error) {
	// A limit of zero or less would never equal len(names), turning the page bound off.
	if limit <= 0 {
		return nil, nil
	}
	limit = min(limit, pageMax)
	var names []string
	scanned := 0
	for ; scanned < scanMax && rows.Scan(); scanned++ {
		name := rows.Text()
		if !strings.HasPrefix(name, namespace+"/") {
			continue
		}
		names = append(names, name)
		if len(names) == limit {
			return names, nil
		}
	}
	// A table of exactly scanMax rows ends cleanly, so probe for one more row.
	if scanned == scanMax && rows.Scan() {
		return names, ErrScanLimit
	}
	// Scan is false for a read error as well as the end, so a failed read
	// must not pass as a short page.
	return names, rows.Err()
}
