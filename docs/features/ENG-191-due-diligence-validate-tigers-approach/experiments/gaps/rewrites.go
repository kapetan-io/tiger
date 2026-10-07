package gaps

import "errors"

// errWalkTooLarge reports a walk that reached nodesMax, so a cycle or an
// oversized graph fails loudly instead of returning a partial walk.
var errWalkTooLarge = errors.New("walk reached nodesMax")

// WalkReported is call 2's compliant worklist: capped, and the cap reports
// when it is hit.
func WalkReported(root *Node) (int, error) {
	stack := []*Node{root}
	i := 0
	for ; i < len(stack) && i < nodesMax; i++ {
		stack = append(stack, stack[i].Next...)
	}
	if i == nodesMax {
		return 0, errWalkTooLarge
	}
	return len(stack), nil
}
