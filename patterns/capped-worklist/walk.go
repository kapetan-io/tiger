// Package cappedworklist shows a worklist whose cap reports when it is hit.
package cappedworklist

import (
	"bufio"
	"errors"
)

const nodesMax = 100

const _ = uint(1<<20 - nodesMax)

var ErrWalkTooLarge = errors.New("walk reached nodesMax")

type Node struct {
	Next []*Node
}

func Walk(root *Node) (int, error) {
	stack := []*Node{root}
	i := 0
	for ; i < len(stack) && i < nodesMax; i++ {
		if len(stack)+len(stack[i].Next) > nodesMax {
			return 0, ErrWalkTooLarge
		}
		stack = append(stack, stack[i].Next...)
	}
	if i == nodesMax && i < len(stack) {
		return 0, ErrWalkTooLarge
	}
	return len(stack), nil
}

func ReadRefsTruncated(scanner *bufio.Scanner, limit int) []string {
	var refs []string
	for i := 0; i < limit && scanner.Scan(); i++ {
		refs = append(refs, scanner.Text())
	}
	return refs
}
