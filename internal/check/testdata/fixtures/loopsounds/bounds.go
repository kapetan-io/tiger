package loopsounds

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"slices"
)


const nodesMax = 1000

const _ = uint(1<<20 - nodesMax)



var errWalkTooLarge = errors.New("walk reached nodesMax")


type Node struct {
	Next []*Node
}



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


var errSpinCapHit = errors.New("spin reached its limit")



func Spin(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		return errSpinCapHit
	}
	return nil
}


var errRetryCapHit = errors.New("retry reached triesMax")


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




func ReadLines(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit && scanner.Scan(); n++ {
		lines = append(lines, scanner.Text())
	}
	return lines
}


func CountRows(rows *sql.Rows, limit int) int {
	total := 0
	for count := 0; count < limit && rows.Next(); count++ {
		total++
	}
	return total
}


func Search(numbers []int) int {
	for i := 0; i < len(numbers); i++ {
		if numbers[i]%2 == 0 {
			return numbers[i]
		}
	}
	return -1
}



func Columns(types *sql.ColumnType, columns []string) int {
	total := 0
	for i := 0; i < len(columns); i++ {
		total += len(columns[i])
	}
	_ = types
	return total
}


func RunUntilCancelled(ctx context.Context, frame func()) {
	for ctx.Err() == nil {
		frame()
	}
}


func Values(numbers []int) int {
	total := 0
	for number := range slices.Values(numbers) {
		total += number
	}
	return total
}
