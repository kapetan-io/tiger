// Unpinned TS-V01 loops whose variant the analyzer synthesizes from the four
// shapes beyond a fixed step: a division or shift by a constant under a
// strict > 0 condition, slicing by the length of a part that is not empty, a
// struct field in the condition, and an && condition where one side shrinks.
package fixture

// varint counts the bytes of a varint, shifting seven bits per pass.
func varint(size uint64) int {
	bytes := 1
	for size > 0 { // want `TS-V01: this loop in varint ends because size shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant size`
		size >>= 7
		bytes++
	}
	return bytes
}

// halvings divides a positive number by two until it reaches zero.
func halvings(value int) int {
	count := 0
	for value > 0 { // want `TS-V01: this loop in halvings ends because value shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant value`
		value /= 2
		count++
	}
	return count
}

// frameMax is the largest packet payload.
const frameMax = 65515

// frames cuts data into packets of at most frameMax bytes.
func frames(data []byte) int {
	packets := 0
	for len(data) > 0 { // want `TS-V01: this loop in frames ends because len\(data\) shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant len\(data\)`
		chunk := data
		if len(chunk) > frameMax {
			chunk = chunk[:frameMax]
		}
		data = data[len(chunk):]
		packets++
	}
	return packets
}

// whole takes all of data each pass, so the part is the whole and not empty.
func whole(data []byte) int {
	packets := 0
	for len(data) >= 1 { // want `TS-V01: this loop in whole ends because len\(data\) shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant len\(data\)`
		chunk := data
		data = data[len(chunk):]
		packets++
	}
	return packets
}

// queue holds items waiting to be handled.
type queue struct{ items []int }

// popAll pops the queue's items one at a time.
func popAll(q *queue) int {
	popped := 0
	for len(q.items) > 0 { // want `TS-V01: this loop in popAll ends because len\(q.items\) shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant len\(q.items\)`
		q.items = q.items[1:]
		popped++
	}
	return popped
}

// backtrack walks two cursors toward the origin; either one alone ends it.
func backtrack(x, y int) int {
	steps := 0
	for x > 0 && y > 0 { // want `TS-V01: this loop in backtrack ends because x shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant x`
		x--
		y--
		steps++
	}
	return steps
}

// pollWhileBusy ends because pending shrinks, whatever busy answers.
func pollWhileBusy(pending []int, busy func() bool) int {
	polled := 0
	for busy() && len(pending) > 0 { // want `TS-V01: this loop in pollWhileBusy ends because len\(pending\) shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant len\(pending\)`
		pending = pending[1:]
		polled++
	}
	return polled
}

// stepPastTop steps by one toward a limit below the type's top.
func stepPastTop(n uint8) int {
	count := 0
	i := uint8(0)
	for i < 200 { // want `TS-V01: this loop in stepPastTop ends because 200 - i shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant 200 - i`
		i++
		count += int(n)
	}
	return count
}
