// A local channel in a package-level function literal is the literal's own,
// closed by a goroutine it starts.
package fixture

var waitForLocalClose = func() {
	done := make(chan struct{})
	go func() { close(done) }()
	<-done
}
