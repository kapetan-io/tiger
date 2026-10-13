// The TS-V01 proofs that look right by name and are wrong by identity,
// plus the loops a sound counter used to hide from this rule.
package fixture

import "math"

// shadowSlice shrinks a copy that shadows data, so data never changes.
func shadowSlice(data []byte) {
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		data := data
		data = data[1:]
		_ = data
	}
}

// aliasAppend grows data through a pointer taken before the loop.
func aliasAppend(data []int) {
	alias := &data
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		*alias = append(*alias, 1)
		data = data[1:]
	}
}

// closureGrow grows data through a closure made before the loop.
func closureGrow(data []int) {
	grow := func() { data = append(data, 1) }
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		grow()
		data = data[1:]
	}
}

// labeledContinue skips the shrink with a continue from an inner loop.
func labeledContinue(data []int) {
outer:
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		for j := 0; j < 1; j++ {
			continue outer
		}
		data = data[1:]
	}
}

// overflow never ends because i wraps past math.MaxInt.
func overflow() {
	n := math.MaxInt
	i := 0
	for i <= n { // want `TS-V01: tiger can't prove this loop ends`
		i++
	}
}

// byteWraps counts a uint8 to 255 with <=, and 255 + 1 wraps to 0.
func byteWraps() {
	for i := uint8(0); i <= 255; i++ { // want `TS-V01: tiger can't prove this loop ends`
	}
}

// unboundedChunk cuts a part of max bytes, which is empty when max is zero.
func unboundedChunk(data []byte, max int) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		chunk := data
		if len(chunk) > max {
			chunk = chunk[:max]
		}
		data = data[len(chunk):]
		count++
	}
	return count
}

// reassignedChunk replaces the part with something that may be empty.
func reassignedChunk(data []byte, other []byte) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		chunk := data
		chunk = other
		data = data[len(chunk):]
		count++
	}
	return count
}

// divideNegative divides by two with a condition that lets zero through.
func divideNegative(value int) int {
	count := 0
	for value >= 0 { // want `TS-V01: tiger can't prove this loop ends`
		value /= 2
		count++
	}
	return count
}

// shiftByVariable shifts by an amount that may be zero.
func shiftByVariable(size uint64, bits uint) int {
	count := 0
	for size > 0 { // want `TS-V01: tiger can't prove this loop ends`
		size >>= bits
		count++
	}
	return count
}

// divideByOne divides by a constant that never makes progress.
func divideByOne(value int) int {
	count := 0
	for value > 0 { // want `TS-V01: tiger can't prove this loop ends`
		value /= 1
		count++
	}
	return count
}

// negatedShrink negates the condition, which does not count.
func negatedShrink(size uint64) int {
	count := 0
	for !(size == 0) { // want `TS-V01: tiger can't prove this loop ends`
		size >>= 1
		count++
	}
	return count
}

// handler holds items and has a method that may add more.
type handler struct{ items []int }

func (h *handler) refill() { h.items = append(h.items, 1) }

// fieldWithCall calls a method on the struct that holds the field.
func fieldWithCall(h *handler) {
	for len(h.items) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		h.items = h.items[1:]
		h.refill()
	}
}

// fieldThroughAlias shrinks the field through the struct and grows it through
// another name for the same struct.
func fieldThroughAlias(h *handler) {
	grow := func() { h.items = append(h.items, 1) }
	for len(h.items) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		h.items = h.items[1:]
		grow()
	}
}

// wrongCounter moves its counter away from the limit.
func wrongCounter(n int) {
	for i := 0; i < n; i-- { // want `TS-V01: tiger can't prove this loop ends`
	}
}

// counterUndone cancels its own counter in the body.
func counterUndone(n int) {
	for i := 0; i < n; i++ { // want `TS-V01: tiger can't prove this loop ends`
		i--
	}
}

// limitGrows raises the limit as fast as the counter climbs.
func limitGrows(n int) {
	for i := 0; i < n; i++ { // want `TS-V01: tiger can't prove this loop ends`
		n++
	}
}

// growsThroughHelper grows the slice whose length is the limit through a
// helper that takes its address.
func growsThroughHelper() {
	stack := []int{0}
	for i := 0; i < len(stack); i++ { // want `TS-V01: tiger can't prove this loop ends`
		push(&stack, i)
	}
}

func push(stack *[]int, value int) { *stack = append(*stack, value) }

// worklistGrows is the worklist TS-S01 recommends, with the cycle it never
// leaves.
func worklistGrows(root *node) int {
	stack := []*node{root}
	for i := 0; i < len(stack); i++ { // want `TS-V01: tiger can't prove this loop ends`
		stack = append(stack, stack[i].next)
	}
	return len(stack)
}

// jumpOver skips the shrink with a goto.
func jumpOver(data []int) {
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		if len(data) > 5 {
			goto next
		}
		data = data[1:]
	next:
	}
}

// wrongPin pins a ranking that grows, because i moves away from n.
func wrongPin(n int) int {
	steps := 0
	i := 0
	//tiger:variant n - i
	for i < n { // want `TS-V01: //tiger:variant n - i doesn't provably shrink on every pass \(i decreases, so the expression grows instead of shrinking\)`
		i--
		steps++
	}
	return steps
}

// pinnedField pins a field that a call may refill.
func pinnedField(h *handler) {
	//tiger:variant len(h.items)
	for len(h.items) > 0 { // want `TS-V01: //tiger:variant len\(h.items\) doesn't provably shrink on every pass`
		h.items = h.items[1:]
		h.refill()
	}
}

// pinnedUnknownName pins a name that is not in scope at the loop.
func pinnedUnknownName(pending []int) {
	//tiger:variant len(missing)
	for len(pending) > 0 { // want `TS-V01: //tiger:variant len\(missing\) doesn't provably shrink on every pass \(len\(missing\) isn't a variable, field or length in scope\)`
		pending = pending[1:]
	}
}
