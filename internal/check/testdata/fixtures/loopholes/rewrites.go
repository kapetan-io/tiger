package loopholes

import "errors"



var errWalkTooLarge = errors.New("walk reached nodesMax")



func WalkReported(root *Node) (int, error) {
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
