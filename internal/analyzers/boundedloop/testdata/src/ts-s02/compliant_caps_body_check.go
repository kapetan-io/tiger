// A page size whose only breaks are stream checks in the body passes the
// same as the header form.
package fixture

import "bufio"

// readLinesLabeledBreak ends the loop with a labeled break on the drained
// stream.
func readLinesLabeledBreak(scanner *bufio.Scanner, limit int) []string {
	var lines []string
scan:
	for n := 0; n < limit; n++ {
		if !scanner.Scan() {
			break scan
		}
		lines = append(lines, scanner.Text())
	}
	return lines
}

// readLinesSwitchBreak has a break inside a switch, which leaves the switch
// and not the loop.
func readLinesSwitchBreak(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit; n++ {
		if !scanner.Scan() {
			break
		}
		switch text := scanner.Text(); text {
		case "":
			break
		default:
			lines = append(lines, text)
		}
	}
	return lines
}

// readLinesTwoDrains checks the stream twice, each check a drain.
func readLinesTwoDrains(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit; n++ {
		if !scanner.Scan() {
			break
		}
		lines = append(lines, scanner.Text())
		if !scanner.Scan() {
			break
		}
		lines = append(lines, scanner.Text())
	}
	return lines
}

// readLinesElseBreak ends on the stream in an else branch.
func readLinesElseBreak(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit; n++ {
		if scanner.Scan() {
			lines = append(lines, scanner.Text())
		} else {
			break
		}
	}
	return lines
}
