package callinventory

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

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
	inv := Inventory{
		Functions: map[string]Function{},
		Packages:  map[string]bool{},
		Roots:     map[string]string{},
		Above:     map[string]string{},
	}
	module := mainModule(roots)
	if module == nil {
		if len(roots) > 0 {
			return Inventory{}, fmt.Errorf("no Go module found for %s", dir)
		}
		return inv, nil
	}
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
		underRun: map[string]bool{},
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
	for _, header := range slices.Sorted(maps.Keys(inv.Functions)) {
		fn := inv.Functions[header]
		for _, key := range slices.Sorted(maps.Keys(fn.Calls)) {
			call := fn.Calls[key]
			call.Sites = orderedSites(call.Sites)
			fn.Calls[key] = call
		}
	}
	slices.Sort(inv.Generated)
	slices.Sort(inv.NotCompared)
	slices.Sort(inv.Unloaded)
	return inv, nil
}

func mainModule(roots []*packages.Package) *packages.Module {
	for _, pkg := range roots {
		if pkg.Module != nil {
			return pkg.Module
		}
	}
	return nil
}

type loader struct {
	run      string
	module   string
	prefix   string
	fset     *token.FileSet
	inv      *Inventory
	reaches  map[string]bool
	body     *bodyReach
	compared map[string]bool
	skipped  map[string]bool
	underRun map[string]bool
	names    map[string]string
	err      error
}

type pkgRef struct {
	Path string
	Name string
}

func refOf(pkg *types.Package) pkgRef {
	return pkgRef{Path: pkg.Path(), Name: pkg.Name()}
}

type callTarget struct {
	Member string
	Method string
}

func (l *loader) inModule(path string) bool {
	return path == l.module || strings.HasPrefix(path, l.module+"/")
}

func (l *loader) classify(roots []*packages.Package) {
	var failures []loadFailure
	for _, pkg := range roots {
		l.underRun[pkg.PkgPath] = true
	}
	packages.Visit(roots, nil, func(pkg *packages.Package) {
		for _, each := range pkg.Errors {
			failures = append(failures, loadFailure{pkg: pkg.PkgPath, message: each.Error()})
		}
		if !l.inModule(pkg.PkgPath) {
			return
		}
		name := l.packageName(pkgRef{Path: pkg.PkgPath, Name: pkg.Name})
		if l.underRun[pkg.PkgPath] {
			if other, taken := l.names[name]; taken && other != pkg.PkgPath {
				l.err = cmp.Or(l.err, fmt.Errorf(
					"two packages print as %s (%s and %s)", name, other, pkg.PkgPath))
			}
			l.names[name] = pkg.PkgPath
		}
		if pkg.IllTyped || len(pkg.Errors) > 0 {
			l.inv.NotCompared = append(l.inv.NotCompared, name)
			l.inv.Unloaded = append(l.inv.Unloaded, pkg.PkgPath)
			return
		}
		l.inv.Packages[name] = true
		if !l.underRun[pkg.PkgPath] {
			l.inv.Above[name] = pkg.PkgPath
			return
		}
		l.inv.Roots[name] = pkg.PkgPath
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

const maxClosures = 1_000_000

var errTooManyClosures = fmt.Errorf("a function holds more than %d function literals", maxClosures)

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

type declaration struct {
	Owner  string
	Header string
	Method string
}

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
			Path:        from.Path(),
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
				found.Sites = append(record.Calls[found.Key()].Sites, l.site(instr))
				record.Calls[found.Key()] = found
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

func (l *loader) site(instr ssa.Instruction) Site {
	pos := l.fset.PositionFor(instr.Pos(), false)
	rel, err := filepath.Rel(l.run, pos.Filename)
	if err != nil {
		rel = pos.Filename
	}
	return Site{File: filepath.ToSlash(rel), Line: pos.Line}
}

func orderedSites(sites []Site) []Site {
	slices.SortFunc(sites, func(a, b Site) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})
	return slices.Compact(sites)
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

func (l *loader) callee(call *ssa.CallCommon, from *types.Package, owner string) (Call, bool) {
	if call.IsInvoke() {
		named, ok := types.Unalias(call.Value.Type()).(*types.Named)
		if !ok {
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
	target := callTarget{Member: obj.Name()}
	if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
		if named := receiver(sig.Recv().Type()); named != nil {
			target = callTarget{Member: named.Obj().Name(), Method: obj.Name()}
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

func (l *loader) other(pkg *types.Package, target callTarget) Call {
	name := l.qualified(pkg, target.Member)
	call := Call{
		Callee:   name,
		Target:   name,
		Symbol:   target.Member,
		Method:   cmp.Or(target.Method, target.Member),
		Package:  l.packageName(refOf(pkg)),
		Path:     pkg.Path(),
		InModule: l.inModule(pkg.Path()),
	}
	if target.Method != "" {
		call.Callee = name + "." + target.Method
	}
	return call
}

func (l *loader) sibling(pkg *types.Package, target callTarget, owner string) (Call, bool) {
	call := l.other(pkg, target)
	if call.Target == owner {
		return Call{}, false
	}
	call.SamePackage = true
	return call, true
}

func (l *loader) keep(callee *ssa.Function, pkg *types.Package) bool {
	if callee.Signature.Recv() == nil {
		found, err := l.body.reaches(callee)
		l.err = cmp.Or(l.err, err)
		return found
	}
	return l.reaches[pkg.Path()]
}
