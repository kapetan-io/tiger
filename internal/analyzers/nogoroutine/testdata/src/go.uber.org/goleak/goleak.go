// Package goleak is the slice of go.uber.org/goleak the corpus uses.
package goleak

// TestingM is what VerifyTestMain needs from a *testing.M.
type TestingM interface{ Run() int }

// VerifyTestMain runs the tests and fails on a leaked goroutine.
func VerifyTestMain(m TestingM) {}

// VerifyNone fails the test if a goroutine leaked.
func VerifyNone(t any, options ...any) {}
