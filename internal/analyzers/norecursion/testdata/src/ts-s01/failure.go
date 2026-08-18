// The TS-S01 failure modes: direct self-recursion, mutual recursion, method
// recursion on a concrete type, and recursion through a statically-known
// function literal.
package fixture

// Node is one node of a tree walked by chase and chaseIterative.
type Node struct {
	Children []*Node
}

// chase walks the tree depth-first by calling itself on every child — its
// stack depth is bounded only by the tree's depth, which comes from data.
func chase(n *Node, visit func(*Node)) { // want `TS-S01: chase → chase`
	if n == nil {
		return
	}
	visit(n)
	for _, child := range n.Children {
		chase(child, visit)
	}
}

// Even and Odd decide parity by calling each other down to zero — mutual
// recursion, still a cycle in the static call graph.
func Even(n int) bool { // want `TS-S01: Even → Odd → Even`
	if n == 0 {
		return true
	}
	return Odd(n - 1)
}

func Odd(n int) bool {
	if n == 0 {
		return false
	}
	return Even(n - 1)
}

// List is a linked list walked by Sum and SumIterative.
type List struct {
	val  int
	next *List
}

// Sum recurses down the linked list one node at a time — a method calling
// itself on a concrete receiver is still a self-edge.
func (l *List) Sum() int { // want `TS-S01: Sum → Sum`
	if l == nil {
		return 0
	}
	return l.val + l.next.Sum()
}

// runA calls itself through a directly-invoked local closure: helper is
// never stored anywhere its target could be ambiguous, so the call through
// it is statically known, and the cycle it forms is real.
func runA() { // want `TS-S01: runA → runA\$1 → runA`
	helper := func() {
		runA()
	}
	helper()
}
