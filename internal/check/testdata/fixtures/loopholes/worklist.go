package loopholes


type Node struct {
	Next []*Node
}




func Walk(root *Node) int {
	stack := []*Node{root}
	for i := 0; i < len(stack); i++ {
		stack = append(stack, stack[i].Next...)
	}
	return len(stack)
}
