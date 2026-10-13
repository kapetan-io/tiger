package gaps

// WrongCounter moves its counter away from the limit. TS-S02 sees a counter,
// and TS-V01 skips counter loops.
func WrongCounter(n int) {
	for i := 0; i < n; i-- {
	}
}

// CounterUndone cancels its own counter in the body.
func CounterUndone(n int) {
	for i := 0; i < n; i++ {
		i--
	}
}

// LimitGrows raises a plain integer limit as fast as the counter climbs.
func LimitGrows(n int) {
	for i := 0; i < n; i++ {
		n++
	}
}

func push(stack *[]int, value int) { *stack = append(*stack, value) }

// HelperGrow grows the worklist through a helper, so an append check on the
// loop body alone misses it.
func HelperGrow() {
	stack := []int{0}
	for i := 0; i < len(stack); i++ {
		push(&stack, i)
	}
}

// MapGrow grows the map whose length is the limit.
func MapGrow(entries map[int]int) {
	for i := 0; i < len(entries); i++ {
		entries[i+len(entries)] = i
	}
}
