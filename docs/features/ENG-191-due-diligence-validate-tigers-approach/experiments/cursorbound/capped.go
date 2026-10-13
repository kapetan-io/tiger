package cursorbound

// ListCapped restates the query's limit in the loop header, so the loop runs
// at most limit times whatever the database returns.
func ListCapped(rows Rows, limit int) []string {
	var names []string
	for count := 0; count < limit && rows.Next(); count++ {
		names = append(names, rows.Value())
	}
	return names
}
