// Package fixture provides a small type with a method so the TS-S13 corpus
// can exercise a bare method call as a qualifying use.
package fixture

type counter struct{ n int }

func (c *counter) Inc() { c.n++ }

func newCounter() *counter { return &counter{} }
