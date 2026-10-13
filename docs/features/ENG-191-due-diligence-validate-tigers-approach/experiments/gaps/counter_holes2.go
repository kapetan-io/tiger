package gaps

// OrCounter puts the counter on one side of ||, so the other side alone keeps
// the loop going. TS-S02 only checks that the counter appears in the condition.
func OrCounter(done func() bool) {
	for i := 0; i < 10 || !done(); i++ {
	}
}

// CounterReset sets the counter back to zero in the body.
func CounterReset() {
	for i := 0; i < 10; i++ {
		i = 0
	}
}

type register struct{ i int }

// FieldNamedLikeCounter compares a field that shares the counter's name. The
// counter advances; the field never moves.
func FieldNamedLikeCounter(r register) {
	for i := 0; r.i < 10; i++ {
	}
}

// StepsPastLimit steps by 2 toward an odd limit with !=, so it never lands on it.
func StepsPastLimit() {
	for i := 0; i != 7; i += 2 {
	}
}

// ByteWraps counts a uint8 to 255 with <=, and 255 + 1 wraps to 0.
func ByteWraps() {
	for i := uint8(0); i <= 255; i++ {
	}
}

// Register returns a register whose i field stays at zero.
func Register() register { return register{} }
