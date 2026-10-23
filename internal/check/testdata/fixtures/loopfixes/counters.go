package loopfixes

// OrCounter keeps the counter in a top-level term the body leaves alone.
func OrCounter(done func() bool) int {
	hits := 0
	for i := 0; i < 10; i++ {
		if done() {
			hits++
		}
	}
	return hits
}

// CounterReset leaves the counter to the post statement.
func CounterReset() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}

// WrongWay steps toward the limit.
func WrongWay(n int) int {
	steps := 0
	for i := 0; i < n; i++ {
		steps++
	}
	return steps
}

// LimitGrows leaves the limit alone.
func LimitGrows(n int) {
	for i := 0; i < n; i++ {
		_ = i
	}
}

// StepsPastLimit uses an ordering comparison, which a step of 2 cannot skip.
func StepsPastLimit() {
	for i := 0; i < 7; i += 2 {
		_ = i
	}
}

// ByteWraps stops one short of the byte's maximum.
func ByteWraps() {
	for i := uint8(0); i < 255; i++ {
		_ = i
	}
}

// Overflow counts to a small constant.
func Overflow() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}

// register holds a limit nothing writes.
type register struct{ limit int }

// FieldNamedLikeCounter compares the counter with a field nothing writes.
func FieldNamedLikeCounter(r register) {
	for i := 0; i < r.limit; i++ {
		_ = i
	}
}
