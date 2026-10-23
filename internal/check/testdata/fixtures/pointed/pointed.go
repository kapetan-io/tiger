// Package pointed starts a goroutine nobody owns, a finding that a pattern backs.
package pointed

// Start launches work with no owner.
func Start(work func()) {
	go work()
}
