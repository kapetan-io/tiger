package annotations

// CountNoReason uses the annotation with no reason text.
func CountNoReason(rows Rows) int {
	count := 0
	//tiger:batched
	for rows.Next() {
		count++
	}
	return count
}
