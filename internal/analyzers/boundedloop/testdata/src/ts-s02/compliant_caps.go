// The TS-S02 caps that report: a safety cap fails when it is hit, and a page
// size over a stream from outside the module needs no report.
package fixture

import (
	"bufio"
	"database/sql"
	"errors"

	"github.com/kapetan-io/tiger/assert"
)

// errSpinCapHit reports a spin that reached its limit before done.
var errSpinCapHit = errors.New("spin reached its limit")

// spinReported returns an error when the cap is hit.
func spinReported(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		return errSpinCapHit
	}
	return nil
}

// walkReported is the capped worklist: it fails when nodes remain unvisited
// at the cap, and a graph of exactly nodesMax nodes still passes.
func walkReported(root *graph) (int, error) {
	stack := []*graph{root}
	i := 0
	for ; i < len(stack) && i < nodesMax; i++ {
		stack = append(stack, stack[i].next...)
	}
	if i == nodesMax && i < len(stack) {
		return 0, errSpinCapHit
	}
	return len(stack), nil
}

// retryFlag fails when the tries run out without success.
func retryFlag(try func() bool) error {
	ok := false
	for attempt := 0; attempt < 5; attempt++ {
		if try() {
			ok = true
			break
		}
	}
	if !ok {
		return errSpinCapHit
	}
	return nil
}

// spinAsserted asserts what the loop was waiting for.
func spinAsserted(done func() bool, limit int) {
	for i := 0; i < limit && !done(); i++ {
	}
	assert.Ok(done(), "spin reached its limit")
}

// readLines reads at most limit lines. Its only other way out drains a
// scanner, a type declared outside the module, so limit is a page size and
// reaching it means the page is full.
func readLines(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit && scanner.Scan(); n++ {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// countRows is the same page size over a database cursor.
func countRows(rows *sql.Rows, limit int) int {
	total := 0
	for n := 0; n < limit && rows.Next(); n++ {
		total++
	}
	return total
}

// plainCount has no other way out, so reaching the limit is how it ends.
func plainCount(limit int) int {
	total := 0
	for i := 0; i < limit; i++ {
		total++
	}
	return total
}

// readLinesBodyCheck is readLines with the stream check in the body, which
// ends the loop the same way.
func readLinesBodyCheck(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit; n++ {
		if !scanner.Scan() {
			break
		}
		lines = append(lines, scanner.Text())
	}
	return lines
}

const (
	pageMax = 1_000
	scanMax = 10_000
)

const _ = uint(scanMax - pageMax)

var errScanLimit = errors.New("list read scanMax rows before the page filled")

// filteredPage counts kept rows against a clamped page limit and every row
// read against scanMax, and fails when scanMax ends the scan.
func filteredPage(rows *sql.Rows, keep func() bool, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	limit = min(limit, pageMax)
	kept, scanned := 0, 0
	for ; scanned < scanMax && rows.Next(); scanned++ {
		if !keep() {
			continue
		}
		kept++
		if kept == limit {
			return kept, nil
		}
	}
	if scanned == scanMax {
		return kept, errScanLimit
	}
	return kept, nil
}
