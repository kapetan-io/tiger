// Package nolint carries two //nolint comments and nothing else for tiger to report.
package nolint

// Total sums entries.
func Total(entries []int) int {
	total := 0
	for _, entry := range entries { //nolint:intrange
		total += entry
	}
	return total
}

func unused() {} // nolint
