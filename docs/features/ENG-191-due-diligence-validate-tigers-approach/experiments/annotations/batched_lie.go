package annotations

// CountEndless carries the same annotation, but rows here never runs out.
// Tiger accepts it because it checks the loop's shape and that a reason
// exists, never what the reason says.
func CountEndless(rows Rows) int {
	count := 0
	//tiger:batched rows come from SELECT ... LIMIT $1
	for rows.Next() {
		count++
	}
	return count
}
