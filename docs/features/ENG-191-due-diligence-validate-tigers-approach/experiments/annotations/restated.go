package annotations

// ListRestated needs no annotation. The loop header holds its own counter
// and limit, so it runs at most limit times whatever rows does.
func ListRestated(rows Rows, limit int) []string {
	var names []string
	for count := 0; count < limit && rows.Next(); count++ {
		names = append(names, rows.Value())
	}
	return names
}
