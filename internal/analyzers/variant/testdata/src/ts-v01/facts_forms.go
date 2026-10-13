// Unpinned TS-V01 loops whose variant is synthesized from the long forms of
// a move, from a move in the post statement, and from branches that only
// look like they skip it.
package fixture

// longForm writes the shift and the division out in full.
func longForm(size uint64, value int) int {
	count := 0
	for size > 0 { // want `TS-V01: this loop in longForm ends because size shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant size`
		size = size >> 7
		count++
	}
	for value > 0 { // want `TS-V01: this loop in longForm ends because value shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant value`
		value = value / 10
		count++
	}
	return count
}

// moveInPost shrinks the slice in the post statement.
func moveInPost(data []int) int {
	count := 0
	for ; len(data) > 0; data = data[1:] { // want `TS-V01: this loop in moveInPost ends because len\(data\) shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant len\(data\)`
		count++
	}
	return count
}

// trimFromEnd drops the last element.
func trimFromEnd(data []int) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: this loop in trimFromEnd ends because len\(data\) shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant len\(data\)`
		data = data[:len(data)-1]
		count++
	}
	return count
}

// continueInInnerLoop continues an inner loop, which does not skip the shrink.
func continueInInnerLoop(data []int) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: this loop in continueInInnerLoop ends because len\(data\) shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant len\(data\)`
		data = data[1:]
		for j := 0; j < 3; j++ {
			if j == 1 {
				continue
			}
			count++
		}
	}
	return count
}

// continueInClosure continues inside a function literal, which ends the
// literal's call and not the loop's pass.
func continueInClosure(data []int) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: this loop in continueInClosure ends because len\(data\) shrinks on every pass — nothing to fix; to make tiger fail the build if that changes, add //tiger:variant len\(data\)`
		data = data[1:]
		each := func(items []int) {
			for _, item := range items {
				if item == 0 {
					continue
				}
				count++
			}
		}
		each(data)
	}
	return count
}
