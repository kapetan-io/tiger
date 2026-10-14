package fixture

// Restarter jumps back to its own loop's label, which restarts the loop and
// does not leave it.
type Restarter struct {
	stop     chan struct{}
	requests chan func()
}

// Stop ends the loop.
func (r *Restarter) Stop() { close(r.stop) }

// Run restarts on a nil request.
func (r *Restarter) Run() {
retry:
	for {
		select {
		case work := <-r.requests:
			if work == nil {
				goto retry
			}
			work()
		case <-r.stop:
			return
		}
	}
}
