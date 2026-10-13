// Values that reach a loop limit by a path a shallow trace skips: a bare
// return of a named result, an explicitly instantiated generic call, a huge
// constant in a literal list, and clamps that a later change undoes.
package fixture

import "lib"

// namedBare hands back its named result on a bare return.
func namedBare(limit int) (n int) { // want namedBare:`\[0\]`
	n = limit
	if limit > 5 {
		return min(limit, 5)
	}
	return
}

// SpinFromNamedBare passes a request value through a function whose bare
// return is unbounded.
func SpinFromNamedBare(request Tuning, done func() bool) error {
	return lib.SpinReported(done, namedBare(request.Spins)) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// spinGeneric loops to a limit and takes a value of any type.
func spinGeneric[T any](done func() bool, limit int, extra T) error { // want spinGeneric:`\[1\]`
	_ = extra
	return lib.SpinReported(done, limit)
}

// SpinInstantiated names the type argument, which hides nothing from the trace.
func SpinInstantiated(request Tuning, done func() bool) error {
	return spinGeneric[int](done, request.Spins, 0) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinEachHugeConstant loops to a constant no loop reaches, held in a list.
func SpinEachHugeConstant(done func() bool) {
	for _, limit := range []int{1 << 62} { // want `TS-S02: this value is too large for the loop to ever reach`
		_ = lib.SpinReported(done, limit)
	}
}

// setting holds a limit.
type setting struct{ limit int }

// take answers differently each time it is asked.
func (s *setting) take() int { return s.limit }

// SpinClampedInsideALoop clamps before the loop, then replaces the value on
// every pass.
func SpinClampedInsideALoop(first int, rest []int, done func() bool) { // want SpinClampedInsideALoop:`\[0\]`
	limit := first
	if limit > spinsMax {
		return
	}
	for _, next := range rest { // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
		_ = lib.SpinReported(done, limit)
		limit = next
	}
}

// SpinClampedThenSetThroughPointer clamps a field, then writes the whole
// struct through a pointer.
func SpinClampedThenSetThroughPointer(p *setting, other setting, done func() bool) {
	if p.limit > spinsMax {
		return
	}
	*p = other
	_ = lib.SpinReported(done, p.limit) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// replace overwrites a setting.
func replace(p *setting, other setting) { *p = other }

// SpinClampedThenReplacedByACall clamps a field, then hands the struct to a
// function that overwrites it.
func SpinClampedThenReplacedByACall(p *setting, other setting, done func() bool) {
	if p.limit > spinsMax {
		return
	}
	replace(p, other)
	_ = lib.SpinReported(done, p.limit) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinClampedCallResult clamps one call's answer and uses the next call's.
func SpinClampedCallResult(p *setting, done func() bool) {
	if p.take() > spinsMax {
		return
	}
	_ = lib.SpinReported(done, p.take()) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinClampedThenDoubled clamps before a loop that doubles the value on every
// pass.
func SpinClampedThenDoubled(first int, done func() bool) { // want SpinClampedThenDoubled:`\[0\]`
	limit := first
	if limit > spinsMax {
		return
	}
	for i := 0; i < 3; i++ {
		_ = lib.SpinReported(done, limit)
		limit = limit * 2 // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	}
}
