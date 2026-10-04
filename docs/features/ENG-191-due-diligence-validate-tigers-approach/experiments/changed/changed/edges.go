// Package changed prototypes `tiger changed <base>`: which owners in a Go
// module started or stopped talking to another package between two versions.
//
// An owner is the named type whose method holds a call site, or the free
// function itself. A call site inside a closure belongs to the function the
// closure sits in. An edge runs from an owner to what it calls in another
// package: a module package's type or function, or an outside package's type
// or function when that package reaches syscall. Calls through an interface
// name the interface. Reach is not propagated: when B starts calling the
// database, the edge appears on B, never on A, which calls B.
package changed

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Filter decides which outside packages count as talking to the world.
type Filter int

const (
	// ByPackage keeps an outside callee when its package is syscall or
	// imports syscall, directly or transitively.
	ByPackage Filter = iota
	// ByFunction keeps an outside callee when the function itself statically
	// reaches package syscall. Interface calls fall back to ByPackage, since
	// an interface method has no body to follow.
	ByFunction
	// Mixed judges an outside free function by its body, as ByFunction does,
	// and an outside method or interface by its package, as ByPackage does.
	// fmt.Errorf and errors.New are free functions that never reach syscall;
	// os.File.Close is a method whose path to syscall a static walk loses.
	Mixed
)

// Edge is one owner talking to one target. Calls holds each "Caller calls
// Method" pair behind it, so a reader can find the edge in the diff.
type Edge struct {
	Owner  string
	Target string
	Calls  []string
}

// Edges is every edge of one module version, keyed by owner then target.
type Edges map[string]map[string]map[string]bool

// Load computes the edges of the module in dir. Test files are left out.
func Load(dir string, filter Filter) (Edges, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.LoadAllSyntax | packages.NeedModule,
		Dir:  dir,
	}, "./...")
	if err != nil {
		return nil, err
	}
	var problems []string
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, e := range pkg.Errors {
			problems = append(problems, e.Error())
		}
	})
	if len(problems) > 0 {
		return nil, fmt.Errorf("loading %s: %s", dir, strings.Join(problems, "; "))
	}
	if len(pkgs) == 0 || pkgs[0].Module == nil {
		return nil, fmt.Errorf("no module found in %s", dir)
	}

	syscall := map[string]bool{}
	var reaches func(pkg *packages.Package) bool
	reaches = func(pkg *packages.Package) bool {
		if done, ok := syscall[pkg.PkgPath]; ok {
			return done
		}
		syscall[pkg.PkgPath] = false
		found := pkg.PkgPath == "syscall"
		for _, imp := range pkg.Imports {
			if reaches(imp) {
				found = true
			}
		}
		syscall[pkg.PkgPath] = found
		return found
	}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) { reaches(pkg) })

	prog, _ := ssautil.AllPackages(pkgs, 0)
	prog.Build()

	walk := &walker{
		module:  pkgs[0].Module.Path,
		syscall: syscall,
		filter:  filter,
		bodies:  map[*ssa.Function]int{},
		edges:   Edges{},
	}
	for fn := range ssautil.AllFunctions(prog) {
		pkg := top(fn).Package()
		if fn.Origin() != nil || pkg == nil || !walk.inModule(pkg.Pkg) {
			continue
		}
		walk.function(fn)
	}
	return walk.edges, nil
}

type walker struct {
	module  string
	syscall map[string]bool
	filter  Filter
	bodies  map[*ssa.Function]int
	edges   Edges
}

func (w *walker) inModule(pkg *types.Package) bool {
	return pkg != nil && (pkg.Path() == w.module || strings.HasPrefix(pkg.Path(), w.module+"/"))
}

// name renders a package-qualified name: module packages relative to the
// module, the root package by its package name, outside packages in full.
func (w *walker) name(pkg *types.Package, member string) string {
	switch {
	case pkg.Path() == w.module:
		return pkg.Name() + "." + member
	case w.inModule(pkg):
		return strings.TrimPrefix(pkg.Path(), w.module+"/") + "." + member
	}
	return pkg.Path() + "." + member
}

// top returns the declared function a closure sits in.
func top(fn *ssa.Function) *ssa.Function {
	for fn.Parent() != nil {
		fn = fn.Parent()
	}
	return fn
}

// function records the edges of every call site in fn.
func (w *walker) function(fn *ssa.Function) {
	decl := top(fn)
	pkg := decl.Package().Pkg
	owner, caller := w.owner(decl)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			target, method, ok := w.target(call.Common(), pkg)
			if !ok {
				continue
			}
			if w.edges[owner] == nil {
				w.edges[owner] = map[string]map[string]bool{}
			}
			if w.edges[owner][target] == nil {
				w.edges[owner][target] = map[string]bool{}
			}
			w.edges[owner][target][caller+" calls "+method] = true
		}
	}
}

// owner names the owner of a declared function and the function itself.
func (w *walker) owner(fn *ssa.Function) (string, string) {
	pkg := fn.Package().Pkg
	if recv := fn.Signature.Recv(); recv != nil {
		if named := receiver(recv.Type()); named != nil {
			return w.name(pkg, named.Obj().Name()), fn.Name()
		}
	}
	// init#1 and the package initializer both belong to the package's init.
	name, _, _ := strings.Cut(fn.Name(), "#")
	return w.name(pkg, name), name
}

func receiver(t types.Type) *types.Named {
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := types.Unalias(t).(*types.Named); ok {
		return named.Origin()
	}
	return nil
}

// target names what a call site talks to, or reports false when the call
// stays in its own package, is dynamic, or is filtered out.
func (w *walker) target(call *ssa.CallCommon, from *types.Package) (string, string, bool) {
	if call.IsInvoke() {
		named, ok := types.Unalias(call.Value.Type()).(*types.Named)
		if !ok {
			// A type parameter or an interface literal names no interface.
			return "", "", false
		}
		named = named.Origin()
		pkg := named.Obj().Pkg()
		if pkg == nil || pkg == from {
			return "", "", false
		}
		if !w.inModule(pkg) && !w.syscall[pkg.Path()] {
			return "", "", false
		}
		return w.name(pkg, named.Obj().Name()), call.Method.Name(), true
	}
	callee := call.StaticCallee()
	if callee == nil {
		return "", "", false
	}
	if callee.Origin() != nil {
		callee = callee.Origin()
	}
	obj, ok := callee.Object().(*types.Func)
	if !ok || obj.Pkg() == nil || obj.Pkg() == from {
		return "", "", false
	}
	if !w.inModule(obj.Pkg()) && !w.keep(callee, obj.Pkg()) {
		return "", "", false
	}
	sig := obj.Type().(*types.Signature)
	if sig.Recv() != nil {
		if named := receiver(sig.Recv().Type()); named != nil {
			return w.name(obj.Pkg(), named.Obj().Name()), obj.Name(), true
		}
	}
	return w.name(obj.Pkg(), obj.Name()), obj.Name(), true
}

func (w *walker) keep(callee *ssa.Function, pkg *types.Package) bool {
	method := callee.Signature.Recv() != nil
	if w.filter == ByFunction || (w.filter == Mixed && !method) {
		return w.body(callee)
	}
	return w.syscall[pkg.Path()]
}

const (
	visiting = iota + 1
	without
	with
)

// body reports whether fn statically reaches package syscall, following
// static calls, referenced functions and closures, but not interface calls.
func (w *walker) body(fn *ssa.Function) bool {
	switch w.bodies[fn] {
	case visiting, without:
		return false
	case with:
		return true
	}
	w.bodies[fn] = visiting
	if pkg := fn.Package(); pkg != nil && (pkg.Pkg.Path() == "syscall" || strings.HasPrefix(pkg.Pkg.Path(), "internal/syscall")) {
		w.bodies[fn] = with
		return true
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			for _, op := range instr.Operands(nil) {
				if op == nil || *op == nil {
					continue
				}
				if next, ok := (*op).(*ssa.Function); ok && w.body(next) {
					w.bodies[fn] = with
					return true
				}
			}
		}
	}
	for _, anon := range fn.AnonFuncs {
		if w.body(anon) {
			w.bodies[fn] = with
			return true
		}
	}
	w.bodies[fn] = without
	return false
}

// List returns the edges sorted by owner then target.
func (e Edges) List() []Edge {
	var list []Edge
	for owner, targets := range e {
		for target, calls := range targets {
			list = append(list, Edge{Owner: owner, Target: target, Calls: sorted(calls)})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Owner != list[j].Owner {
			return list[i].Owner < list[j].Owner
		}
		return list[i].Target < list[j].Target
	})
	return list
}

func sorted(set map[string]bool) []string {
	list := make([]string, 0, len(set))
	for item := range set {
		list = append(list, item)
	}
	sort.Strings(list)
	return list
}
