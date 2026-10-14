// Documented TS-S03 coverage gaps: recognizing a stop case by what the package
// does with the channel can still be fooled by code that never runs.
package fixture

// Dormant closes its channel in code that can never run.
type Dormant struct {
	stop  chan struct{}
	input chan int
}

// Stop would end the loop, but only when its condition holds.
func (d *Dormant) Stop() {
	if false {
		close(d.stop)
	}
}

// Run runs the loop on the calling goroutine.
//
// known-miss: the close in Stop is dead code, but it is syntactically a
// close an exported method reaches, so the stop case counts and the loop
// that can never stop goes unreported. Seeing that the branch never runs
// needs reachability analysis this check does not have; the goleak harness
// and shutdown tests are the runtime backstop.
func (d *Dormant) Run(output chan<- int) {
	for {
		select {
		case <-d.stop:
			return
		case value := <-d.input:
			output <- value * 2
		}
	}
}
