// Package pointed starts a goroutine nobody owns, a finding that a pattern backs.
package pointed


func Start(work func()) {
	go work()
}
