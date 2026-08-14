// The compliant rewrites: work submitted through a callback-style Go
// method carries no GoStmt, and both an allowlisted function and an
// allowlisted method stay silent even though each starts a goroutine
// directly.
package fixture

// group is a stand-in for errgroup.Group: work is submitted through Go,
// never spawned directly.
type group struct{}

func (group) Go(fn func()) {
	fn()
}

func viaGroup() {
	var g group
	g.Go(work)
}

// Supervise is on the test's -supervisors allowlist as ts-c02.Supervise.
func Supervise() {
	go work()
}

// Runner is a supervisor type; Run is allowlisted as ts-c02.Runner.Run.
type Runner struct{}

func (Runner) Run() {
	go work()
}
