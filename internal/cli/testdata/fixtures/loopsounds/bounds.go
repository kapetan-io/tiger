package loopsounds

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"slices"
)

// nodesMax is the most nodes a walk visits.
const nodesMax = 1000

const _ = uint(1<<20 - nodesMax)

// errWalkTooLarge reports a walk that reached nodesMax, which on a graph
// with a cycle is the only way it ends.
var errWalkTooLarge = errors.New("walk reached nodesMax")

// Node is a graph node whose edges may lead back to an ancestor.
type Node struct {
	Next []*Node
}

// Walk visits every node reachable from root, the worklist TS-S01 recommends
// in place of recursion, and fails on a cycle instead of running on.
func Walk(root *Node) (int, error) {
	stack := []*Node{root}
	i := 0
	for ; i < len(stack) && i < nodesMax; i++ {
		stack = append(stack, stack[i].Next...)
	}
	if i == nodesMax && i < len(stack) {
		return 0, errWalkTooLarge
	}
	return len(stack), nil
}

// errSpinCapHit reports a spin that reached its limit before done.
var errSpinCapHit = errors.New("spin reached its limit")

// Spin calls done until it reports true, and fails when the limit passes
// first.
func Spin(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		return errSpinCapHit
	}
	return nil
}

// errRetryCapHit reports a retry that used every try.
var errRetryCapHit = errors.New("retry reached triesMax")

// Retry runs try until it succeeds, and fails when the tries run out.
func Retry(try func() bool) error {
	const triesMax = 5
	ok := false
	for attempt := 0; attempt < triesMax; attempt++ {
		if try() {
			ok = true
			break
		}
	}
	if !ok {
		return errRetryCapHit
	}
	return nil
}

// ReadLines reads at most limit lines. Its only other way out drains a
// scanner from outside the module, so limit is a page size and hitting it
// needs no report.
func ReadLines(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit && scanner.Scan(); n++ {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// CountRows counts at most limit rows from a query.
func CountRows(rows *sql.Rows, limit int) int {
	total := 0
	for count := 0; count < limit && rows.Next(); count++ {
		total++
	}
	return total
}

// Search finds the first even number, stopping at the end of the slice.
func Search(numbers []int) int {
	for i := 0; i < len(numbers); i++ {
		if numbers[i]%2 == 0 {
			return numbers[i]
		}
	}
	return -1
}

// Columns walks a result's columns by the count the result reports, the way
// go/types code walks a tuple.
func Columns(types *sql.ColumnType, columns []string) int {
	total := 0
	for i := 0; i < len(columns); i++ {
		total += len(columns[i])
	}
	_ = types
	return total
}

// RunUntilCancelled draws frames until the caller cancels ctx.
func RunUntilCancelled(ctx context.Context, frame func()) {
	for ctx.Err() == nil {
		frame()
	}
}

// Values sums an iterator from the standard library.
func Values(numbers []int) int {
	total := 0
	for number := range slices.Values(numbers) {
		total += number
	}
	return total
}
