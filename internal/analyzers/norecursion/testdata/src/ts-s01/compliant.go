// The TS-S01 compliant rewrites: an explicit stack for the tree walk, a
// bounded queue with an iteration cap and an assert on exhaustion for the
// linked-list walk, a single loop replacing mutual recursion, and an
// index-advancing worklist replacing the closure-mediated cycle. None of
// these call themselves, directly or through another member, so the static
// call graph they induce has no cycle.
package fixture

import "assert"

// listStepsMax bounds sumBounded's walk down the linked list.
const listStepsMax = 1000

// chaseIterative walks the tree with an explicit stack — an
// index-advancing worklist — instead of calling itself.
func chaseIterative(root *Node, visit func(*Node)) {
	if root == nil {
		return
	}
	stack := []*Node{root}
	for i := 0; i < len(stack); i++ {
		visit(stack[i])
		stack = append(stack, stack[i].Children...)
	}
}

// parity replaces the Even/Odd mutual recursion with one loop: no call back
// into a caller, so no cycle.
func parity(n int) bool {
	even := true
	for i := 0; i < n; i++ {
		even = !even
	}
	return even
}

// sumBounded replaces List.Sum's self-recursion with an iteration cap and an
// assert on exhaustion — the loop terminates within listStepsMax rounds or
// the assert fires, so the walk's worst case is stated, not implied.
func sumBounded(l *List) int {
	total := 0
	cur := l
	for i := 0; i < listStepsMax; i++ {
		if cur == nil {
			return total
		}
		total += cur.val
		cur = cur.next
	}
	assert.Ok(false, "list exceeds listStepsMax")
	return total
}

// runAIterative replaces the closure-mediated cycle with a worklist the
// closure feeds but never calls back into: process only ever returns more
// work, it does not call runAIterative.
func runAIterative() {
	const stepsMax = 10
	process := func(step int) []int {
		if step >= stepsMax {
			return nil
		}
		return []int{step + 1}
	}
	steps := []int{0}
	for i := 0; i < len(steps); i++ {
		steps = append(steps, process(steps[i])...)
	}
}
