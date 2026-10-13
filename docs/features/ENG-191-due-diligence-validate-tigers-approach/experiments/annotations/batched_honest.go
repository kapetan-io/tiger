package annotations

// List reads a query that carries LIMIT. The reason text is true.
func List(rows Rows) []string {
	var names []string
	//tiger:batched rows come from SELECT ... LIMIT $1
	for rows.Next() {
		names = append(names, rows.Value())
	}
	return names
}
