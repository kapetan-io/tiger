package fixture

// Chain closes its stop channel through two helpers.
type Chain struct {
	stop   chan struct{}
	values chan int
}

func closeOnce(c chan struct{}) { close(c) }

func closeTwice(c chan struct{}) { closeOnce(c) }

// Shutdown reaches the close through closeTwice and closeOnce.
func (c *Chain) Shutdown() { closeTwice(c.stop) }

// Next waits for a value or the stop.
func (c *Chain) Next() int {
	select {
	case <-c.stop:
		return 0
	case v := <-c.values:
		return v
	}
}
