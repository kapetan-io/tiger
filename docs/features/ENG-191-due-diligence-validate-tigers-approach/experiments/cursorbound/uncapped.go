package cursorbound

// ListUncapped is querator's shape today: the loop runs until the database
// stops sending rows, so its bound lives in the SQL LIMIT that tiger can't see.
func ListUncapped(rows Rows) []string {
	var names []string
	for rows.Next() {
		names = append(names, rows.Value())
	}
	return names
}
