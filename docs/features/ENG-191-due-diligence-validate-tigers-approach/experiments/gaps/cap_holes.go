package gaps

import "math"

// SpinCapped wraps an unprovable condition in a cap no one can reach. Tiger
// accepts it: the counter is the bound, and the cap's size is never checked.
func SpinCapped(done func() bool) {
	for i := 0; i < math.MaxInt && !done(); i++ {
	}
}

const nodesMax = 100

// WalkCapped is call 2's proposed worklist cap. On a cycle it stops at nodesMax
// and returns a partial walk with no error.
func WalkCapped(root *Node) int {
	stack := []*Node{root}
	for i := 0; i < len(stack) && i < nodesMax; i++ {
		stack = append(stack, stack[i].Next...)
	}
	return len(stack)
}

// WalkRange dodges both rules by ranging over the worklist, and range never
// sees the nodes appended during the loop.
func WalkRange(root *Node) int {
	stack := []*Node{root}
	visited := 0
	for _, node := range stack {
		visited++
		stack = append(stack, node.Next...)
	}
	return visited
}
