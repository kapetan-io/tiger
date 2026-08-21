// known-miss: TS-L05 never compares declarations across files. Router's
// type and constructor live in support.go; this method for the same struct
// sits in a different file entirely, so the rule's file-scoped design never
// looks past its own file to find Router's type or constructor, and a
// misplaced method here stays silent no matter how a reader would want it
// reordered relative to support.go's declarations.
package fixture

func (r *Router) Prefix() string {
	return r.prefix
}
