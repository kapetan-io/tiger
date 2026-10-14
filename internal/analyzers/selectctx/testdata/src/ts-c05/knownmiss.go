package fixture

import "context"

// known-miss: ranging over a channel receives on every iteration with no
// shutdown case, but the receive never surfaces as a unary <-expression
// AST node, so selectctx cannot see it. boundedloop owns the shape of an
// unbounded loop, including this one.
func drain(values chan int) int {
	total := 0
	for v := range values {
		total += v
	}
	return total
}

// Request is answered when its Reply channel closes.
type Request struct {
	Reply chan struct{}
}

// Answer closes the reply.
func (r *Request) Answer() { close(r.Reply) }

// Wait blocks on the reply.
//
// known-miss: Reply is closed by the package, so the bare receive counts as a
// wait for shutdown, though it is a completion wait that blocks forever when
// nobody calls Answer. A close in a method nothing calls cannot be told from
// one that runs.
func (r *Request) Wait() {
	<-r.Reply
}

// DeadStop is closed only on a path that never runs.
type DeadStop struct {
	stop chan struct{}
}

// Close would release the select, but only when its condition holds.
func (d *DeadStop) Close() {
	if false {
		close(d.stop)
	}
}

// Wait blocks on stop.
//
// known-miss: the close in Close is dead code, but it is a close an exported
// method reaches, so the select counts as stoppable.
func (d *DeadStop) Wait(ctx context.Context) {
	select {
	case <-d.stop:
	}
}
