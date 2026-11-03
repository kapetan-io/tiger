// Package filteredpage lists one row per line of a scanner, the way a store
// cursor yields rows. It shows a filtered list loop that fills its page and
// reports when it gives up, beside one that returns a short page silently.
package filteredpage

import (
	"bufio"
	"errors"
	"strings"
)

const pageMax = 1_000

const scanMax = 10_000

const _ = uint(scanMax - pageMax)

var ErrScanLimit = errors.New("list read scanMax rows before the page filled")

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

func List(rows *bufio.Scanner, namespace string, limit int) ([]string, error) {
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
	if scanned == scanMax && rows.Scan() {
		return names, ErrScanLimit
	}
	return names, rows.Err()
}
