package gaps

// Node is a graph node whose edges may lead back to an ancestor.
type Node struct {
	Next []*Node
}

// Walk is TS-S01's documented replacement for recursion. The counter chases
// a slice the body grows, so on a cycle the loop never ends, yet tiger reports
// nothing: TS-S02 sees a counter and TS-V01 skips counter loops.
func Walk(root *Node) int {
	stack := []*Node{root}
	for i := 0; i < len(stack); i++ {
		stack = append(stack, stack[i].Next...)
	}
	return len(stack)
}
