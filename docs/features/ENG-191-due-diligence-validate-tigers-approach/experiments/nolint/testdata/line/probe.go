package probe

// Contains has a labeled break, which TS-S09 reports on the break line.
func Contains(grid [][]int, want int) bool {
	found := false
outer:
	for _, row := range grid {
		for _, v := range row {
			if v == want {
				found = true
				break outer //nolint:tiger // the plugin drops TS-S09 here
			}
		}
	}
	return found
}
