// Package callinventory records, for one checkout of a Go module, which
// function makes which calls into other packages. Callees outside the module
// are kept only when they reach package syscall. It knows nothing about git
// or about comparing two checkouts; it loads whatever directory it is given.
package callinventory

import "strings"

// Inventory is one revision's view of a module.
type Inventory struct {
	// Functions holds every function of the compared packages, keyed by its
	// printed header (internal.Daemon.OnStart, internal/storage.ParseOID).
	Functions map[string]Function
	// Packages is the printed name of every module package that loaded, including
	// ones above the run directory; only those under it have Functions.
	Packages map[string]bool
	// Roots is the printed name of every loaded package under the run directory.
	Roots map[string]bool
	// Above maps the printed name of each loaded module package outside the run
	// directory to its import path. Such a name can equal a root's, which only
	// the comparison of two revisions can see.
	Above map[string]string
	// Generated lists the generated files that were skipped, as slash paths
	// relative to the run directory, in byte order.
	Generated []string
	// NotCompared names the module packages that did not load, in byte order.
	NotCompared []string
	// LoadError is the package error that sorts first by package path then message,
	// empty when every package loaded.
	LoadError string
}

// Function is where a call is written: one method of an owner type, or a free
// function. Calls inside closures belong to the enclosing function.
type Function struct {
	Package string
	// Owner is the type holding a method, or the free function itself.
	Owner string
	// Initializer marks init functions and package variable initializers. They
	// are not counted as owners.
	Initializer bool
	// CallsOut is true when the function's own body calls into another package,
	// not only into sibling owners of its own.
	CallsOut bool
	// Calls is keyed by Call.Key.
	Calls map[string]Call
}

// Call is one callee of a function. The pair (function, Callee) is the call's
// identity; nothing about where it is written belongs to that identity.
type Call struct {
	// Callee is the printed name: internal/storage.OID.String or
	// internal/storage.ParseOID.
	Callee string
	// Target is the callee's type or free function, Callee without the method.
	Target string
	// Symbol is Target without its package.
	Symbol  string
	Package string
	// Path is the import path of Package.
	Path string
	// InModule is true when the callee's package belongs to the module.
	InModule bool
	// SamePackage marks a callee that is a function of a different owner in the
	// caller's own package. Whether it counts depends on what the callee itself
	// calls, which the comparison decides.
	SamePackage bool
}

// Key identifies the call within its function. It qualifies the callee by
// import path, so two packages that print the same name stay two calls.
func (c Call) Key() string {
	return c.Path + strings.TrimPrefix(c.Callee, c.Package)
}
