// The compliant rewrite: work submitted through a callback-style Go
// method, the errgroup.Group.Go shape, carries no GoStmt.
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
