// More TS-S02 counter failures: the other direction, a step that is not a
// step, and a limit the body can move through its struct.
package fixture

// towardTheWrongSide counts up while the comparison asks it to stay above.
func towardTheWrongSide(n int) {
	for i := 0; i > n; i++ { // want `TS-S02: this counter does not bound the loop`
	}
}

// downToNotEqual compares with != while counting down, which a negative
// start never meets.
func downToNotEqual(n, stop int) {
	for i := n; i != stop; i-- { // want `TS-S02: this counter does not bound the loop`
	}
}

// zeroStep adds a constant that makes no progress.
func zeroStep(n int) {
	for i := 0; i < n; i += 0 { // want `TS-S02: this counter does not bound the loop`
	}
}

// multiplyStep scales the counter, which is no fixed step.
func multiplyStep(n int) {
	for i := 1; i < n; i *= 1 { // want `TS-S02: `
	}
}

// meter holds a limit that its own methods can move.
type meter struct{ limit int }

func (m *meter) grow() { m.limit++ }

// fieldLimitWritten raises the limit field in the body.
func fieldLimitWritten(m *meter) {
	for i := 0; i < m.limit; i++ { // want `TS-S02: this counter does not bound the loop`
		m.limit++
	}
}

// fieldLimitThroughMethod raises the limit field through a method.
func fieldLimitThroughMethod(m *meter) {
	for i := 0; i < m.limit; i++ { // want `TS-S02: this counter does not bound the loop`
		m.grow()
	}
}

// fieldLimitEscaped raises the limit field through a pointer taken before
// the loop.
func fieldLimitEscaped(m *meter) {
	p := &m.limit
	for i := 0; i < m.limit; i++ { // want `TS-S02: this counter does not bound the loop`
		*p++
	}
}

// limitFromChannel asks a channel for a new limit on every pass.
func limitFromChannel(limits chan int) {
	for i := 0; i < <-limits; i++ { // want `TS-S02: this counter does not bound the loop`
	}
}
