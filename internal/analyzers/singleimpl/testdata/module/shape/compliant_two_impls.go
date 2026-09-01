// Package shape declares an interface with two non-test implementations,
// so TS-X01 stays silent.
package shape

// Shape is implemented twice in this package.
type Shape interface {
	Area() float64
}

// Circle implements Shape.
type Circle struct {
	Radius float64
}

// Area returns the circle's area.
func (c Circle) Area() float64 {
	return c.Radius * c.Radius
}

// Square implements Shape.
type Square struct {
	Side float64
}

// Area returns the square's area.
func (s Square) Area() float64 {
	return s.Side * s.Side
}
