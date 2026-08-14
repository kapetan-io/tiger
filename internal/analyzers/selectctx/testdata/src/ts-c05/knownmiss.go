// known-miss: ranging over a channel receives on every iteration with no
// shutdown case, but the receive never surfaces as a unary <-expression
// AST node, so selectctx cannot see it. boundedloop owns the shape of an
// unbounded loop, including this one.
package fixture

func drain(values chan int) int {
	total := 0
	for v := range values {
		total += v
	}
	return total
}
