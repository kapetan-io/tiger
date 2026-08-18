package dep

// Helper is exported so the probe analyzer can tag it with a cross-package
// fact and prove the dependent package can import that fact back.
func Helper() int {
	return 1
}
