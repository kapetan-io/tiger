package loopfixes

import "slices"

// ShadowSlice shrinks the slice the condition measures.
func ShadowSlice(data []byte) {
	for len(data) > 0 {
		data = data[1:]
	}
}

// AliasAppend never grows data, so the shrink is a proof.
func AliasAppend(data []int) {
	alias := data
	for len(data) > 0 {
		data = data[1:]
	}
	_ = alias
}

// LabeledContinue moves the inner loop into its own function.
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

// GotoLoop counts instead of jumping back.
func GotoLoop() {
	for i := 0; i < 3; i++ {
		_ = i
	}
}

// RangeNaturals ranges over a standard library iterator.
func RangeNaturals(numbers []int) {
	for value := range slices.Values(numbers) {
		_ = value
	}
}

// Rows is the part of a database cursor the loops use.
type Rows interface {
	Next() bool
}

// DrainRefill shrinks pending and never refills it.
func DrainRefill(pending []string) int {
	drained := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		pending = pending[1:]
		drained++
	}
	return drained
}

// DrainSkip shrinks, and counts only what skip lets through.
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

// CountVariant pins a variable the loop shrinks.
func CountVariant(rows Rows, remaining int) int {
	count := 0
	//tiger:variant remaining
	for remaining > 0 && rows.Next() {
		remaining--
		count++
	}
	return count
}
