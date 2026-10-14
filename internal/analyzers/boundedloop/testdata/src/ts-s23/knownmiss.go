// Documented TS-S23 coverage gaps.
package fixture

// Courier takes requests and shutdowns on channels it sends on.
type Courier struct {
	requests   chan func()
	shutdownCh chan struct{}
}

// Submit queues work.
func (c *Courier) Submit(work func()) { c.requests <- work }

// Shutdown asks the loop to stop.
func (c *Courier) Shutdown() { c.shutdownCh <- struct{}{} }

// run returns from the request case unconditionally.
//
// known-miss: a case on a channel the package only sends on counts as the stop
// case when it ends in a return, and the request case does, so the shutdown
// handler is skipped without a report.
func (c *Courier) run() {
	for {
		select {
		case work := <-c.requests:
			work()
			return
		case <-c.shutdownCh:
			return
		}
	}
}
