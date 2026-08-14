package findings

// Sum adds rows, escaping two loops with a label.
func Sum(rows [][]int) int {
	total := 0
scan:
	for _, row := range rows {
		for _, cell := range row {
			if cell < 0 {
				break scan
			}
			total += cell
		}
	}
	return total
}
