package callinventory

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Load records the calls of the Go module that holds dir, for the packages
// under dir. Test files and generated files are left out. A module package
// that does not load is named in NotCompared and contributes nothing; Load
// fails only when the load itself cannot run.
func Load(dir string) (Inventory, error) {
	run, err := filepath.Abs(dir)
	if err != nil {
		return Inventory{}, err
	}
	if run, err = filepath.EvalSymlinks(run); err != nil {
		return Inventory{}, err
	}
	roots, err := packages.Load(&packages.Config{
		Mode: packages.LoadAllSyntax | packages.NeedModule,
		Dir:  run,
	}, "./...")
	if err != nil {
		return Inventory{}, err
	}
	inv := Inventory{Functions: map[string]Function{}, Packages: map[string]bool{}}
	module := mainModule(roots)
	if module == nil {
		if len(roots) > 0 {
			return Inventory{}, fmt.Errorf("no Go module found for %s", dir)
		}
		return inv, nil
	}
	// Both paths are resolved so a symlinked temp directory still compares equal.
	moduleDir, err := filepath.EvalSymlinks(module.Dir)
	if err != nil {
		return Inventory{}, err
	}
	prefix, err := filepath.Rel(moduleDir, run)
	if err != nil {
		return Inventory{}, err
	}
	if prefix == "." {
		prefix = ""
	}
	load := &loader{
		run:      run,
		module:   module.Path,
		prefix:   filepath.ToSlash(prefix),
		inv:      &inv,
		reaches:  importReach(roots),
		body:     newBodyReach(),
		compared: map[string]bool{},
		skipped:  map[string]bool{},
		names:    map[string]string{},
	}
	load.classify(roots)
	prog, _ := ssautil.AllPackages(roots, 0)
	prog.Build()
	load.fset = prog.Fset
	if err := load.walk(prog, roots); err != nil {
		return Inventory{}, err
	}
	if load.err != nil {
		return Inventory{}, load.err
	}
	slices.Sort(inv.Generated)
	slices.Sort(inv.NotCompared)
	return inv, nil
}

// mainModule returns the module that holds the first loaded package.
func mainModule(roots []*packages.Package) *packages.Module {
	for _, pkg := range roots {
		if pkg.Module != nil {
			return pkg.Module
		}
	}
	return nil
}

type loader struct {
	run    string
	module string
	// prefix is the run directory below the module root, empty when they are
	// the same directory.
	prefix  string
	fset    *token.FileSet
	inv     *Inventory
	reaches map[string]bool
	body    *bodyReach
	// compared is the healthy module packages under the run directory, by import path.
	compared map[string]bool
	// skipped is the set of generated files, by absolute path.
	skipped map[string]bool
	// names maps each printed package name to its import path, to catch two
	// packages that would print the same.
	names map[string]string
	// err is the first failure of a bounded walk, which stops the load.
	err error
}

// pkgRef identifies a package by import path and declared name.
type pkgRef struct {
	Path string
	Name string
}

func refOf(pkg *types.Package) pkgRef {
	return pkgRef{Path: pkg.Path(), Name: pkg.Name()}
}

// callTarget is what a call names inside a package: a type or function, and
// the method on it. For a free function the two are the same.
type callTarget struct {
	Member string
	Method string
}

func (l *loader) inModule(path string) bool {
	return path == l.module || strings.HasPrefix(path, l.module+"/")
}

// classify sorts the loaded packages into compared, not compared and
// generated, and remembers the first load error.
func (l *loader) classify(roots []*packages.Package) {
	var failures []loadFailure
	underRun := map[string]bool{}
	for _, pkg := range roots {
		underRun[pkg.PkgPath] = true
	}
	packages.Visit(roots, nil, func(pkg *packages.Package) {
		for _, each := range pkg.Errors {
			failures = append(failures, loadFailure{pkg: pkg.PkgPath, message: each.Error()})
		}
		if !l.inModule(pkg.PkgPath) {
			return
		}
		name := l.packageName(pkgRef{Path: pkg.PkgPath, Name: pkg.Name})
		if other, taken := l.names[name]; taken && other != pkg.PkgPath {
			l.err = cmp.Or(l.err, fmt.Errorf(
				"two packages print as %s (%s and %s)", name, other, pkg.PkgPath))
		}
		l.names[name] = pkg.PkgPath
		if pkg.IllTyped || len(pkg.Errors) > 0 {
			l.inv.NotCompared = append(l.inv.NotCompared, name)
			return
		}
		l.inv.Packages[name] = true
		if !underRun[pkg.PkgPath] {
			return
		}
		l.compared[pkg.PkgPath] = true
		for _, file := range pkg.Syntax {
			if ast.IsGenerated(file) {
				l.skip(pkg.Fset.File(file.Pos()).Name())
			}
		}
	})
	if len(failures) > 0 {
		first := slices.MinFunc(failures, func(a, b loadFailure) int {
			return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.message, b.message))
		})
		l.inv.LoadError = first.message
	}
}

// skip leaves a generated file out and names it in the inventory.
func (l *loader) skip(path string) {
	l.skipped[path] = true
	rel, err := filepath.Rel(l.run, path)
	if err != nil {
		rel = path
	}
	l.inv.Generated = append(l.inv.Generated, filepath.ToSlash(rel))
}

type loadFailure struct {
	pkg     string
	message string
}

// packageName prints a package the way a reviewer finds it: relative to the
// run directory when it sits under it, otherwise relative to the module root,
// outside packages in full. The package at the run directory prints by its
// package name.
func (l *loader) packageName(pkg pkgRef) string {
	if !l.inModule(pkg.Path) {
		return pkg.Path
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(pkg.Path, l.module), "/")
	switch {
	case rel == l.prefix:
		return pkg.Name
	case l.prefix == "":
		return rel
	}
	return strings.TrimPrefix(rel, l.prefix+"/")
}

func (l *loader) qualified(pkg *types.Package, member string) string {
	return l.packageName(refOf(pkg)) + "." + member
}

// maxClosures bounds the function literals one declared function may hold, so
// a malformed program fails the load instead of looping.
const maxClosures = 1_000_000

var errTooManyClosures = fmt.Errorf("a function holds more than %d function literals", maxClosures)

// walk records every function declared in a hand-written file of a compared
// package, and each package's initializer, which holds the calls of package
// variable declarations.
func (l *loader) walk(prog *ssa.Program, roots []*packages.Package) error {
	for _, pkg := range roots {
		built := prog.ImportedPackage(pkg.PkgPath)
		if built == nil || !l.compared[pkg.PkgPath] {
			continue
		}
		for _, file := range pkg.Syntax {
			if l.skipped[l.fset.File(file.Pos()).Name()] {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
				if !ok {
					continue
				}
				if err := l.function(prog.FuncValue(obj)); err != nil {
					return err
				}
			}
		}
		if initializer, ok := built.Members["init"].(*ssa.Function); ok {
			if err := l.function(initializer); err != nil {
				return err
			}
		}
	}
	return nil
}

// declaration names a declared function: its owner, its header and its method
// name, which is the function's own name for a free function.
type declaration struct {
	Owner  string
	Header string
	Method string
}

// declared names a declared function. SSA calls a second init function init#1; it
// files under the package's init with the synthesized package initializer.
func (l *loader) declared(fn *ssa.Function) declaration {
	pkg := fn.Package().Pkg
	if recv := fn.Signature.Recv(); recv != nil {
		if named := receiver(recv.Type()); named != nil {
			owner := l.qualified(pkg, named.Obj().Name())
			return declaration{Owner: owner, Header: owner + "." + fn.Name(), Method: fn.Name()}
		}
	}
	name, _, _ := strings.Cut(fn.Name(), "#")
	owner := l.qualified(pkg, name)
	return declaration{Owner: owner, Header: owner, Method: name}
}

// function records the calls of a declared function and of the function
// literals inside it, all filed under the declared function.
func (l *loader) function(decl *ssa.Function) error {
	if decl == nil {
		return nil
	}
	from := decl.Package().Pkg
	site := l.declared(decl)
	record, ok := l.inv.Functions[site.Header]
	if !ok {
		record = Function{
			Package:     l.packageName(refOf(from)),
			Owner:       site.Owner,
			Initializer: decl.Signature.Recv() == nil && site.Method == "init",
			Calls:       map[string]Call{},
		}
	}
	queue := []*ssa.Function{decl}
	i := 0
	for ; i < len(queue) && i < maxClosures; i++ {
		for _, block := range queue[i].Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(ssa.CallInstruction)
				if !ok || l.skipped[l.fset.Position(instr.Pos()).Filename] {
					continue
				}
				found, ok := l.callee(call.Common(), from, site.Owner)
				if !ok {
					continue
				}
				record.Calls[found.Callee] = found
				record.CallsOut = record.CallsOut || !found.SamePackage
			}
		}
		queue = append(queue, queue[i].AnonFuncs...)
	}
	if i == maxClosures && i < len(queue) {
		return errTooManyClosures
	}
	l.inv.Functions[site.Header] = record
	return nil
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

// callee names what one call site talks to, or reports false when the call
// is dynamic, filtered out, or stays inside its own owner.
func (l *loader) callee(call *ssa.CallCommon, from *types.Package, owner string) (Call, bool) {
	if call.IsInvoke() {
		named, ok := types.Unalias(call.Value.Type()).(*types.Named)
		if !ok {
			// A type parameter or an interface literal names no interface.
			return Call{}, false
		}
		named = named.Origin()
		pkg := named.Obj().Pkg()
		if pkg == nil || pkg.Path() == from.Path() {
			return Call{}, false
		}
		if !l.inModule(pkg.Path()) && !l.reaches[pkg.Path()] {
			return Call{}, false
		}
		target := callTarget{Member: named.Obj().Name(), Method: call.Method.Name()}
		return l.other(pkg, target), true
	}
	static := call.StaticCallee()
	if static == nil {
		return Call{}, false
	}
	if static.Origin() != nil {
		static = static.Origin()
	}
	obj, ok := static.Object().(*types.Func)
	if !ok || obj.Pkg() == nil {
		return Call{}, false
	}
	target := callTarget{Member: obj.Name(), Method: obj.Name()}
	if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
		if named := receiver(sig.Recv().Type()); named != nil {
			target.Member = named.Obj().Name()
		}
	}
	if obj.Pkg().Path() == from.Path() {
		return l.sibling(from, target, owner)
	}
	if !l.inModule(obj.Pkg().Path()) && !l.keep(static, obj.Pkg()) {
		return Call{}, false
	}
	return l.other(obj.Pkg(), target), true
}

// other builds a callee of the first kind: a function, type or interface in
// another package.
func (l *loader) other(pkg *types.Package, target callTarget) Call {
	name := l.qualified(pkg, target.Member)
	call := Call{
		Callee:   name,
		Target:   name,
		Symbol:   target.Member,
		Package:  l.packageName(refOf(pkg)),
		InModule: l.inModule(pkg.Path()),
	}
	if target.Method != target.Member {
		call.Callee = name + "." + target.Method
	}
	return call
}

// sibling builds a callee of the second kind, a function of a different owner
// in the caller's own package. A call within one owner is no callee at all.
func (l *loader) sibling(pkg *types.Package, target callTarget, owner string) (Call, bool) {
	call := l.other(pkg, target)
	if call.Target == owner {
		return Call{}, false
	}
	call.SamePackage = true
	return call, true
}

// keep decides whether an outside callee talks to the world. A free function is
// judged by its own body. A method is judged by whether its package imports
// syscall, because a static walk cannot follow os.File.Close down to it.
func (l *loader) keep(callee *ssa.Function, pkg *types.Package) bool {
	if callee.Signature.Recv() == nil {
		found, err := l.body.reaches(callee)
		l.err = cmp.Or(l.err, err)
		return found
	}
	return l.reaches[pkg.Path()]
}
