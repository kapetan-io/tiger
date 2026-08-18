// The compliant order: type, then constructor, then methods; and a struct
// with no constructor at all, where type-then-methods is enough.
package fixture

type Gauge struct {
	value int
}

func NewGauge(value int) *Gauge {
	return &Gauge{value: value}
}

func (g *Gauge) Value() int {
	return g.value
}

// Counter has no constructor: type-then-method is a complete, compliant
// order on its own.
type Counter struct {
	n int
}

func (c *Counter) Increment() {
	c.n++
}

// Counter's second method, appearing after the first, stays silent too —
// only a declaration that precedes something it must follow ever fires.
func (c *Counter) Reset() {
	c.n = 0
}
