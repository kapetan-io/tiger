// A package whose non-test files start no goroutine needs no leak check, even
// when its tests start one.
package fixture

// Add sums two numbers.
func Add(a, b int) int { return a + b }
