// want package:`closed-dispatch, no-reflect, imports\(fixture\.example/ts-p02bare, fixture\.example/ts-p02helper\)`
package tsp02

// UseNothing declares no restriction of its own but participates in no
// TS-P02 line either, since only a package with a claim can have that
// claim weakened.
func UseNothing() int {
	return 1
}
