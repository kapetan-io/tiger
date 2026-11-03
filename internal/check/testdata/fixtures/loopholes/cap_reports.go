package loopholes

import (
	"bufio"
	"errors"
)


var errSpinCapHit = errors.New("spin reached its limit")




func ReadLines(scanner *bufio.Scanner, limit int) []string {
	var lines []string
	for n := 0; n < limit && scanner.Scan(); n++ {
		lines = append(lines, scanner.Text())
	}
	return lines
}



func SpinBreak(done func() bool, limit int) {
	for i := 0; i < limit; i++ {
		if done() {
			break
		}
	}
}


type Spinner struct {
	Max int
}


func (s Spinner) Spin(done func() bool) {
	for i := 0; i < s.Max && !done(); i++ {
	}
}



func SpinReported(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		return errSpinCapHit
	}
	return nil
}


type endlessReader struct{}

func (endlessReader) Read(buffer []byte) (int, error) {
	for i := range buffer {
		buffer[i] = '\n'
	}
	return len(buffer), nil
}



func ScanForever(limit int) int {
	scanner := bufio.NewScanner(endlessReader{})
	n := 0
	for ; n < limit && scanner.Scan(); n++ {
	}
	return n
}
