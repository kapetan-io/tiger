package loopholes



func WrongCounter(n int) {
	for i := 0; i < n; i-- {
	}
}


func CounterUndone(n int) {
	for i := 0; i < n; i++ {
		i--
	}
}


func LimitGrows(n int) {
	for i := 0; i < n; i++ {
		n++
	}
}

func push(stack *[]int, value int) { *stack = append(*stack, value) }



func HelperGrow() {
	stack := []int{0}
	for i := 0; i < len(stack); i++ {
		push(&stack, i)
	}
}


func MapGrow(entries map[int]int) {
	for i := 0; i < len(entries); i++ {
		entries[i+len(entries)] = i
	}
}
