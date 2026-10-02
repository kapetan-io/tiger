package repo

// FindOld was committed before the branch. Its labeled break is a TS-S09
// finding that --new-from-rev hides.
func FindOld(grid [][]int, want int) bool {
	found := false
outer:
	for _, row := range grid {
		for _, v := range row {
			if v == want {
				found = true
				break outer
			}
		}
	}
	return found
}
