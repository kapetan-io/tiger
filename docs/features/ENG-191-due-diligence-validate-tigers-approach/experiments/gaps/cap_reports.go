package gaps

import (
	"bufio"
	"errors"
)

// errSpinCapHit reports a spin that reached its limit before done.
var errSpinCapHit = errors.New("spin reached its limit")

// ReadLines reads at most limit lines. Its only other exit drains a
// bufio.Scanner, a stream from outside the module, so limit is a page size
// and hitting it needs no report.
func ReadLines(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit && scanner.Scan(); n++ {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// SpinBreak exits through a break on an internal predicate, so its counter is
// a safety cap and must report when hit, wherever its limit comes from.
func SpinBreak(done func() bool, limit int) {
	for i := 0; i < limit; i++ {
		if done() {
			break
		}
	}
}

// Spinner keeps its cap in a field.
type Spinner struct {
	Max int
}

// Spin is SpinLimited with the limit read from a field.
func (s Spinner) Spin(done func() bool) {
	for i := 0; i < s.Max && !done(); i++ {
	}
}

// SpinReported is SpinLimited with the report: it returns an error when the
// cap is hit, so a cap reached by mistake fails loudly.
func SpinReported(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		return errSpinCapHit
	}
	return nil
}

// endlessReader never returns io.EOF.
type endlessReader struct{}

func (endlessReader) Read(buffer []byte) (int, error) {
	for i := range buffer {
		buffer[i] = '\n'
	}
	return len(buffer), nil
}

// ScanForever wraps a reader that never ends in a bufio.Scanner, which counts
// as an outside stream, so its limit needs no report.
func ScanForever(limit int) int {
	scanner := bufio.NewScanner(endlessReader{})
	n := 0
	for ; n < limit && scanner.Scan(); n++ {
	}
	return n
}
