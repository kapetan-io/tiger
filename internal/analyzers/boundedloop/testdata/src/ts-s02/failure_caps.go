// The TS-S02 caps that stop without saying so: a counter that stands in for a
// proof, because the loop has another way out, must report when it is hit.
package fixture

// spinConstant wraps an unprovable condition in a cap no one reports.
func spinConstant(done func() bool) {
	for i := 0; i < 1000 && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
}

// spinParameter takes its cap as a parameter.
func spinParameter(done func() bool, limit int) {
	for i := 0; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
}

// pacer keeps its cap in a field.
type pacer struct{ max int }

// spin is spinParameter with the limit read from a field.
func (p pacer) spin(done func() bool) {
	for i := 0; i < p.max && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
}

// spinBreak leaves through a break, so its counter is a safety cap too.
func spinBreak(done func() bool, limit int) {
	for i := 0; i < limit; i++ { // want `TS-S02: this counter cap ends the loop without saying so`
		if done() {
			break
		}
	}
}

// graph is a node whose edges may lead back to an ancestor.
type graph struct{ next []*graph }

const nodesMax = 100

const _ = uint(1000 - nodesMax)

// walkCapped stops at nodesMax and returns a partial walk with no error.
func walkCapped(root *graph) int {
	stack := []*graph{root}
	for i := 0; i < len(stack) && i < nodesMax; i++ { // want `TS-S02: this counter cap ends the loop without saying so`
		stack = append(stack, stack[i].next...)
	}
	return len(stack)
}

// pageOfConstant limits a stream by a constant, which is no page size: the
// constant is a safety cap on the scanner.
func pageOfConstant(next func() bool) int {
	n := 0
	for ; n < 10 && next(); n++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	return n
}

// reportsOutsideTheLoop checks something unrelated to the loop afterwards, so
// the cap is still silent.
func reportsOutsideTheLoop(done func() bool, limit int, ready bool) error {
	for i := 0; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	if !ready {
		return errNotReady
	}
	return nil
}

type notReady struct{}

func (notReady) Error() string { return "not ready" }

var errNotReady error = notReady{}
