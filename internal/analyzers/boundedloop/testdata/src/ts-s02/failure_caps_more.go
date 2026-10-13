// More TS-S02 caps that stop without saying so: a constant cap over an
// outside stream, a stream from this module, a break beside an outside
// stream, a cap written with the counter on the right, and reports that do
// not report.
package fixture

import (
	"database/sql"
	"errors"
)

// pageOfConstantRows caps a database cursor at a constant, which is a safety
// cap and not a page size.
func pageOfConstantRows(rows *sql.Rows) int {
	n := 0
	for ; n < 10 && rows.Next(); n++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	return n
}

// localRows is a cursor declared in this module.
type localRows struct{ left int }

func (r *localRows) Next() bool { r.left--; return r.left > 0 }

// pageOverLocalStream drains a stream declared in this module, so the limit
// is a safety cap.
func pageOverLocalStream(rows *localRows, limit int) int {
	n := 0
	for ; n < limit && rows.Next(); n++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	return n
}

// pageWithBreak drains an outside stream and also breaks, so the limit is
// not the only reason the loop stops.
func pageWithBreak(rows *sql.Rows, limit int, enough func() bool) int {
	n := 0
	for ; n < limit && rows.Next(); n++ { // want `TS-S02: this counter cap ends the loop without saying so`
		if enough() {
			break
		}
	}
	return n
}

// capOnTheRight writes the comparison with the limit first.
func capOnTheRight(done func() bool, limit int) {
	for i := 0; limit > i && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
}

// returnsNilAtTheCap returns nil, which reports nothing.
func returnsNilAtTheCap(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if i == limit {
		return nil
	}
	return nil
}

// returnsNumberAtTheCap returns a number where the error would be.
func returnsNumberAtTheCap(done func() bool, limit int) int {
	i := 0
	for ; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if i == limit {
		return i
	}
	return 0
}

// cleansUpAtTheCap does something at the cap, but nothing that fails.
func cleansUpAtTheCap(done func() bool, limit int, cleanup func()) {
	i := 0
	for ; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if i == limit {
		cleanup()
	}
}

// downToVariableCap counts down by 2 with another way out and says nothing
// when the counter runs out.
func downToVariableCap(start, floor int, done func() bool) {
	for i := start; i > floor && !done(); i -= 2 { // want `TS-S02: this counter cap ends the loop without saying so`
	}
}

// checksTheCounterAgainstZero fails on a condition that never compares the
// counter with the limit, so the cap stays silent. The post statement writes
// the counter, which is not the loop waiting for a result.
func checksTheCounterAgainstZero(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if i == 0 {
		return errors.New("empty")
	}
	return nil
}

// lessThanTheLimit fails when the loop stopped early. At the cap the counter
// equals the limit, so the comparison is false and the cap stays silent.
func lessThanTheLimit(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if i < limit {
		return errors.New("stopped early")
	}
	return nil
}

// limitAboveTheCounter is the same comparison with the operands swapped.
func limitAboveTheCounter(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if limit > i {
		return errors.New("stopped early")
	}
	return nil
}

// pastTheLimitAfterAnExclusiveLoop compares with greater-than, which is
// false when the counter stops at the limit.
func pastTheLimitAfterAnExclusiveLoop(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if i > limit {
		return errors.New("past the limit")
	}
	return nil
}

// steppedPastTheLimit steps by 2, so an odd limit is jumped over and equality
// never holds.
func steppedPastTheLimit(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i += 2 { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if i == limit {
		return errors.New("hit the limit")
	}
	return nil
}
