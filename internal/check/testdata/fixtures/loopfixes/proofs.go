package loopfixes

import "slices"


func ShadowSlice(data []byte) {
	for len(data) > 0 {
		data = data[1:]
	}
}


func AliasAppend(data []int) {
	alias := data
	for len(data) > 0 {
		data = data[1:]
	}
	_ = alias
}


func LabeledContinue(data []int) {
	for len(data) > 0 {
		data = data[1:]
		once()
	}
}

func once() {
	for j := 0; j < 1; j++ {
		return
	}
}


func GotoLoop() {
	for i := 0; i < 3; i++ {
		_ = i
	}
}


func RangeNaturals(numbers []int) {
	for value := range slices.Values(numbers) {
		_ = value
	}
}


type Rows interface {
	Next() bool
}


func DrainRefill(pending []string) int {
	drained := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		pending = pending[1:]
		drained++
	}
	return drained
}


func DrainSkip(pending []string, skip func() bool) int {
	drained := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		pending = pending[1:]
		if !skip() {
			drained++
		}
	}
	return drained
}


func CountVariant(rows Rows, remaining int) int {
	count := 0
	//tiger:variant remaining
	for remaining > 0 && rows.Next() {
		remaining--
		count++
	}
	return count
}
