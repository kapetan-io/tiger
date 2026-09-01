package dep // want "probe: finished fixture.example/matching/dep"

// Helper is imported by app.
func Helper() int {
	return 1
}
