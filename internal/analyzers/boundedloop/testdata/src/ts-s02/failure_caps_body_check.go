// Body breaks that are not a bare stream check leave the limit a safety cap.
package fixture

import "bufio"

// readLinesBreakOnScan breaks when the stream still has data.
func readLinesBreakOnScan(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit; n++ { // want `TS-S02: this counter cap ends the loop without saying so`
		if scanner.Scan() {
			break
		}
		lines = append(lines, scanner.Text())
	}
	return lines
}

// readLinesDrainWithWork does more than break when the stream is drained.
func readLinesDrainWithWork(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit; n++ { // want `TS-S02: this counter cap ends the loop without saying so`
		if !scanner.Scan() {
			lines = append(lines, "end")
			break
		}
		lines = append(lines, scanner.Text())
	}
	return lines
}

// readLinesDrainOrStop folds another condition into the break.
func readLinesDrainOrStop(scanner *bufio.Scanner, limit int, stop func() bool) []string {
	var lines []string
	for n := 0; n < limit; n++ { // want `TS-S02: this counter cap ends the loop without saying so`
		if !scanner.Scan() || stop() {
			break
		}
		lines = append(lines, scanner.Text())
	}
	return lines
}

// readLinesInnerStream drains a stream made inside the module.
func readLinesInnerStream(next func() bool, limit int) []string {
	var lines []string
	for n := 0; n < limit; n++ { // want `TS-S02: this counter cap ends the loop without saying so`
		if !next() {
			break
		}
		lines = append(lines, "x")
	}
	return lines
}

// readLinesLabeledOther breaks out with a label on a non-drain check, after a
// drain check.
func readLinesLabeledOther(scanner *bufio.Scanner, limit int, stop func() bool) []string {
	var lines []string
scan:
	for n := 0; n < limit; n++ { // want `TS-S02: this counter cap ends the loop without saying so`
		if !scanner.Scan() {
			break scan
		}
		for i := 0; i < 3; i++ { // want `TS-S02: this counter cap ends the loop without saying so`
			if stop() {
				break scan
			}
		}
		lines = append(lines, scanner.Text())
	}
	return lines
}
