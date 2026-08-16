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

// drain breaks out of a select inside its only loop: Go forces the label
// because a bare break would leave just the select.
func drain(work chan int) int {
	total := 0
empty:
	for {
		select {
		case n := <-work:
			total += n
		default:
			break empty // want `TS-S09: labeled break escapes a select from inside its only loop`
		}
	}
	return total
}

// classify breaks out of a switch inside its only loop.
func classify(sizes []int) int {
	kept := 0
scan2:
	for _, size := range sizes {
		switch {
		case size < 0:
			break scan2 // want `TS-S09: labeled break escapes a switch from inside its only loop`
		default:
			kept++
		}
	}
	return kept
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
