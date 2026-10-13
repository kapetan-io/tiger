// Values that look unbounded to a trace that reads too little: a function
// literal's parameter, a literal list of constants, a quotient, and a field of
// a struct with no name.
package fixture

import "lib"

// SpinInsideLiteral loops to a function literal's parameter, which is called
// through a function value the trace does not follow.
func SpinInsideLiteral(done func() bool) {
	run := func(limit int) error {
		return lib.SpinReported(done, limit)
	}
	_ = run(5)
}

// SpinEachConstant loops to each constant in a literal list.
func SpinEachConstant(done func() bool) {
	for _, limit := range []int{1, 2, 3} {
		_ = lib.SpinReported(done, limit)
	}
}

// SpinPerStep loops to a length divided by a step, which is no more than the
// length.
func SpinPerStep(items []int, step int, done func() bool) error {
	return lib.SpinReported(done, len(items)/step)
}

// SpinAnonymousField reads a field of a struct that has no name, which keeps
// no fact.
func SpinAnonymousField(done func() bool) {
	pair := struct{ limit int }{limit: 3}
	for i := 0; i < pair.limit; i++ {
		_ = done()
	}
}
