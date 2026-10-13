package annotations

// CountVariant tries to state a variant for a cursor loop. Nothing in the
// program shrinks, so there is no honest expression to write.
func CountVariant(rows Rows, remaining int) int {
	count := 0
	//tiger:variant remaining
	for rows.Next() {
		count++
	}
	return count
}
