// Documented TS-S02 gaps in the write check: a field changed through a name
// the loop never mentions, a limit that a declared function grows, and a
// report that is an unrelated check.
package fixture

import "errors"

// counted holds a count a loop reads.
type counted struct{ n int }

var shared = &counted{n: 5}

func bumpShared() { shared.n += 10 }

// aliasThroughGlobal reads a field that a helper changes through a global.
//
// known-miss: the body passes nothing of type counted to bumpShared, so the
// write through the global pointer is invisible to the check on the field.
func aliasThroughGlobal(p *counted) {
	for i := 0; i < p.n; i++ {
		bumpShared()
	}
}

// bumper changes a counted.
type bumper interface{ bump() }

func (c *counted) bump() { c.n += 10 }

// aliasThroughInterface reads a field that an interface method changes.
//
// known-miss: the call receives an interface, not the struct type that
// owns the field, so the write is invisible to the check on the field.
func aliasThroughInterface(c *counted, b bumper) {
	for i := 0; i < c.n; i++ {
		b.bump()
	}
}

var grown int

func nextLimit() int {
	grown++
	return grown + 10
}

// limitFromGetter asks a declared function for a limit that grows.
//
// known-miss: a declared function or method is trusted to return a limit
// that holds still, because loops over tuple.Len() are everywhere.
func limitFromGetter() {
	for i := 0; i < nextLimit(); i++ {
	}
}

var errUnrelated = errors.New("unrelated")

// capWithUnrelatedCheck checks an error the loop assigns, which has nothing
// to do with the cap.
//
// known-miss: any failing if on a variable the loop assigns counts as the
// report, because a retry loop reports its last error that way.
func capWithUnrelatedCheck(max int, read func() (int, error)) error {
	var err error
	var value int
	for tries := 0; tries < max; tries++ {
		value, err = read()
		if value == 3 {
			break
		}
	}
	if err != nil {
		return errUnrelated
	}
	return nil
}

// growsThroughACopy inserts into a map through a second name for it.
//
// known-miss: the check follows writes to the variable named in the limit,
// and a copy of a map is a new variable for the same map.
func growsThroughACopy(entries map[int]int) {
	copied := entries
	for i := 0; i < len(entries); i++ {
		copied[i+1000] = 1
	}
}
