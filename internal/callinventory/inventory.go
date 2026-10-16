// Package callinventory records, for one checkout of a Go module, which
// function makes which calls into other packages. It knows nothing about git
// or about comparing two checkouts; it loads whatever directory it is given.
package callinventory

// Inventory is one revision's view of a module.
type Inventory struct {
	// Functions holds every function of the compared packages, keyed by its
	// printed header (internal.Daemon.OnStart, internal/storage.ParseOID).
	Functions map[string]Function
	// Packages is the printed name of every module package that loaded.
	Packages map[string]bool
	// Generated lists the generated files that were skipped, as slash paths
	// relative to the run directory, in byte order.
	Generated []string
	// NotCompared names the module packages that did not load, in byte order.
	NotCompared []string
	// LoadError is the first package error met, empty when every package loaded.
	LoadError string
}

// Function is where a call is written: one method of an owner type, or a free
// function. Calls inside closures belong to the enclosing function.
type Function struct {
	Package string
	// Owner is the type holding a method, or the free function itself.
	Owner string
	// Initializer marks init functions and package variable initializers,
	// which are not a type or function a reviewer names.
	Initializer bool
	// CallsOut is true when the function's own body makes at least one call
	// of the first kind (into another package).
	CallsOut bool
	// Calls is keyed by Call.Callee.
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
	// InModule is true when the callee's package belongs to the module.
	InModule bool
	// SamePackage marks the second kind of callee, a function of a different
	// owner in the caller's own package. Whether it counts depends on what the
	// callee itself calls, which the comparison decides.
	SamePackage bool
}
