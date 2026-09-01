package hidden

// TestOnly is declared in a _test.go file, so TS-X01 never sees it —
// singleimpl only counts interfaces declared in non-test files.
type TestOnly interface {
	Ping() bool
}

// onlyImpl is TestOnly's only implementation, also in a _test.go file.
type onlyImpl struct{}

// Ping always succeeds.
func (onlyImpl) Ping() bool { return true }
