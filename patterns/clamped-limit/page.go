package clampedlimit

import "bufio"

// pageMax is the most rows one request may take. It is small enough that a
// full page finishes in well under a second.
const pageMax = 1_000

// pageHuge is a "maximum" no page can reach in useful time. It is a clamp in
// form only: the loop below ends at it, after about a billion rows.
const pageHuge = 1 << 30

// pageHuge must stay above pageMax, so the broken shape stays a real clamp.
const _ = uint(pageHuge - pageMax)

// ListRequest is a list request as decoded off the wire.
type ListRequest struct {
	Limit int32
}

// clampPage keeps a requested page size in 1..pageMax. The floor turns zero
// and negative requests into one row, not an empty or runaway page.
func clampPage(requested int) int {
	return max(1, min(requested, pageMax))
}

// countRows counts at most limit rows from a stream.
func countRows(scanner *bufio.Scanner, limit int) (int, error) {
	count := 0
	for ; count < limit && scanner.Scan(); count++ {
	}
	return count, scanner.Err()
}

// ListPage clamps the client's limit where the request enters, then passes
// only the clamped value on.
func ListPage(scanner *bufio.Scanner, req ListRequest) (int, error) {
	return countRows(scanner, clampPage(int(req.Limit)))
}

// ListPageHuge clamps against pageHuge. Tiger sees a clamp against a declared
// maximum and accepts it, but the maximum does not bound the loop in time.
func ListPageHuge(scanner *bufio.Scanner, req ListRequest) (int, error) {
	return countRows(scanner, min(int(req.Limit), pageHuge))
}
