package loopholes

import "math"



func SpinCapped(done func() bool) {
	for i := 0; i < math.MaxInt && !done(); i++ {
	}
}

const nodesMax = 100



func WalkCapped(root *Node) int {
	stack := []*Node{root}
	for i := 0; i < len(stack) && i < nodesMax; i++ {
		stack = append(stack, stack[i].Next...)
	}
	return len(stack)
}



func WalkRange(root *Node) int {
	stack := []*Node{root}
	visited := 0
	for _, node := range stack {
		visited++
		stack = append(stack, node.Next...)
	}
	return visited
}




func SpinLimited(done func() bool, limit int) {
	for i := 0; i < limit && !done(); i++ {
	}
}
