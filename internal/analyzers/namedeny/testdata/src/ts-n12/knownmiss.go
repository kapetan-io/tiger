// known-miss: a range clause's loop variables (RangeStmt.Key/Value) are not
// AssignStmt nodes, so a banned token used there is not walked by the
// declared-identifier collector and stays silent even though it is exactly
// the placeholder-name failure TS-N12 exists to catch.
package fixture

func sumOrders(orders []int) int {
	total := 0
	for _, value := range orders {
		total += value
	}
	return total
}
