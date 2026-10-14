// Package cappedworklist shows a worklist whose cap reports when it is hit.
package cappedworklist

import (
	"bufio"
	"errors"
)

// nodesMax bounds a walk; no real graph is near it, so reaching it means a
// cycle or an attack.
const nodesMax = 100

// Stops compiling if nodesMax outgrows the memory a walk may use.
const _ = uint(1<<20 - nodesMax)

// ErrWalkTooLarge reports a walk that reached nodesMax.
var ErrWalkTooLarge = errors.New("walk reached nodesMax")

// Node is a graph node whose edges may lead back to an ancestor.
type Node struct {
	Next []*Node
}

// Walk counts the nodes of a tree, once per parent edge. A cycle never runs
// out of work, so hitting the cap is an error and never a short count.
func Walk(root *Node) (int, error) {
	stack := []*Node{root}
	i := 0
	for ; i < len(stack) && i < nodesMax; i++ {
		// Check before appending, so one node with a huge edge list cannot
		// allocate past the cap.
		if len(stack)+len(stack[i].Next) > nodesMax {
			return 0, ErrWalkTooLarge
		}
		stack = append(stack, stack[i].Next...)
	}
	// A graph of exactly nodesMax nodes ends here too, with nothing left to visit.
	if i == nodesMax && i < len(stack) {
		return 0, ErrWalkTooLarge
	}
	return len(stack), nil
}

// ReadRefsTruncated is the broken shape. Its other exit drains a bufio.Scanner,
// an outside stream, so tiger takes limit as a page size and asks for no report.
// An advertisement longer than limit returns a short list the caller cannot
// tell from a complete one.
func ReadRefsTruncated(scanner *bufio.Scanner, limit int) []string {
	var refs []string
	for i := 0; i < limit && scanner.Scan(); i++ {
		refs = append(refs, scanner.Text())
	}
	return refs
}
