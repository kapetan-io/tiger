// support.go declares Router's type and constructor; knownmiss.go pairs a
// method for the same struct in a different file to demonstrate TS-L05's
// file-scoped gap.
package fixture

type Router struct {
	prefix string
}

func NewRouter(prefix string) *Router {
	return &Router{prefix: prefix}
}
