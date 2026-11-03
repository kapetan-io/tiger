package clampedlimit

import "bufio"

const pageMax = 1_000

const pageHuge = 1 << 30

const _ = uint(pageHuge - pageMax)

type ListRequest struct {
	Limit int32
}

func clampPage(requested int) int {
	return max(1, min(requested, pageMax))
}

func countRows(scanner *bufio.Scanner, limit int) (int, error) {
	count := 0
	for ; count < limit && scanner.Scan(); count++ {
	}
	return count, scanner.Err()
}

func ListPage(scanner *bufio.Scanner, req ListRequest) (int, error) {
	return countRows(scanner, clampPage(int(req.Limit)))
}

func ListPageHuge(scanner *bufio.Scanner, req ListRequest) (int, error) {
	return countRows(scanner, min(int(req.Limit), pageHuge))
}
