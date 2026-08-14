// The compliant rewrites: plain loops, early returns, and extracted
// functions carry the same control flow with no labels.
package fixture

// retryCompliant is the goto case rewritten as the loop it wanted to be.
func retryCompliant(attempts int) int {
	count := 0
	for count < attempts {
		count++
	}
	return count
}

// firstMatchCompliant extracts the inner loop so a plain return replaces the
// labeled break.
func firstMatchCompliant(rows [][]int, needle int) bool {
	for _, row := range rows {
		if rowContains(row, needle) {
			return true
		}
	}
	return false
}

func rowContains(row []int, needle int) bool {
	for _, cell := range row {
		if cell == needle {
			return true
		}
	}
	return false
}

// unlabeledBranches shows plain break and continue stay silent.
func unlabeledBranches(row []int) int {
	total := 0
	for _, cell := range row {
		if cell == 0 {
			continue
		}
		if cell < 0 {
			break
		}
		total += cell
	}
	return total
}
