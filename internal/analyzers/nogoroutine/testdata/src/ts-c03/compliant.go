// The compliant side of TS-C03: a package whose own files start no goroutine
// needs no leak check, because there is nothing for goleak to find. A package
// that does start goroutines is compliant once a TestMain calls goleak, which
// the ts-c03ok package shows.
package fixture

// Counter adds up values on the calling goroutine.
type Counter struct {
	total int
}

// Add counts one value.
func (c *Counter) Add(value int) { c.total += value }
