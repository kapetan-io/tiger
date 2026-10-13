// More TS-S02 caps that report: any comparison that is true at the limit, a
// panic, and a limit written as a literal where the loop names a constant.
package fixture

import "errors"

var errTooSlow = errors.New("too slow")

// atOrPastTheLimit reports with >=.
func atOrPastTheLimit(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i >= limit {
		return errTooSlow
	}
	return nil
}

// limitOnTheLeftOfTheReport reports with the limit written first.
func limitOnTheLeftOfTheReport(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if limit <= i {
		return errTooSlow
	}
	return nil
}

const triesMax = 5

// literalAtTheReport compares with the literal that triesMax folds to.
func literalAtTheReport(done func() bool) error {
	i := 0
	for ; i < triesMax && !done(); i++ {
	}
	if i == 5 {
		return errTooSlow
	}
	return nil
}

// panicsAtTheCap panics when the cap is hit.
func panicsAtTheCap(done func() bool, limit int) {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		panic("too slow")
	}
}
