// Package callinventory records, for one checkout of a Go module, which
// function makes which calls into other packages. Callees outside the module
// are kept only when they reach package syscall. It knows nothing about git
// or about comparing two checkouts; it loads whatever directory it is given.
package callinventory

import "strings"

type Inventory struct {
	Functions   map[string]Function
	Packages    map[string]bool
	Roots       map[string]string
	Above       map[string]string
	Generated   []string
	NotCompared []string
	Unloaded    []string
	LoadError   string
}

type Function struct {
	Package     string
	Path        string
	Owner       string
	Initializer bool
	CallsOut    bool
	Calls       map[string]Call
}

type Call struct {
	Callee      string
	Target      string
	Symbol      string
	Method      string
	Package     string
	Path        string
	InModule    bool
	SamePackage bool
	Sites       []Site
}

type Site struct {
	File string
	Line int
}

func (c Call) Key() string {
	return c.Path + "\x00" + strings.TrimPrefix(c.Callee, c.Package)
}
