// The TS-S09 failure modes: goto, labeled break, labeled continue.
package fixture

// retry uses goto for a loop that wanted to be a for statement.
func retry(attempts int) int {
	count := 0
begin:
	count++
	if count < attempts {
		goto begin // want `TS-S09: goto transfers control invisibly`
	}
	return count
}

// firstMatch breaks across two loops with a label.
func firstMatch(rows [][]int, needle int) bool {
	found := false
scan:
	for _, row := range rows {
		for _, cell := range row {
			if cell == needle {
				found = true
				break scan // want `TS-S09: labeled break reaches across loops`
			}
		}
	}
	return found
}

// skipEmpty continues an outer loop with a label.
func skipEmpty(rows [][]int) int {
	total := 0
rows:
	for _, row := range rows {
		for _, cell := range row {
			if cell == 0 {
				continue rows // want `TS-S09: labeled continue reaches across loops`
			}
			total += cell
		}
	}
	return total
}
