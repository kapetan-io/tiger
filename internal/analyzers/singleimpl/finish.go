package singleimpl

import (
	"go/types"
	"maps"
	"slices"
	"sort"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/finish"
)

// candidate is one named type eligible to implement an interface.
type candidate struct {
	pkgPath string
	name    string
	named   *types.Named
}

// interfaceEntry is one declared interface eligible for TS-X01.
type interfaceEntry struct {
	pkgPath string
	name    string
	obj     *types.TypeName
	iface   *types.Interface
}

// Finish is TS-X01's whole-program half: for every interface any package's
// Fact declared, it counts how many candidate types across the module
// implement it — as T or *T, counted once — and reports the interfaces
// with exactly one. Zero implementations is not a finding: the interface
// is a contract with no implementer yet, TS-X01's target is speculative
// indirection, not dead code. Candidates and interfaces are each sorted by
// (package path, name) before matching, so the result never depends on
// fact export order.
func Finish(program *finish.Program) ([]analysis.Diagnostic, error) {
	facts := packageFacts(program)
	candidates := resolveCandidates(program, facts)
	interfaces := resolveInterfaces(program, facts)

	diagnostics := []analysis.Diagnostic{}
	for _, entry := range interfaces {
		implementers := implementersOf(entry, candidates)
		if len(implementers) != 1 {
			continue
		}
		diagnostics = append(diagnostics, diagnosticFor(entry, implementers[0]))
	}
	return diagnostics, nil
}

// packageFacts indexes singleimpl's package facts by import path.
func packageFacts(program *finish.Program) map[string]*Fact {
	facts := map[string]*Fact{}
	for _, packageFact := range program.PackageFacts {
		if fact, ok := packageFact.Fact.(*Fact); ok {
			facts[packageFact.Package.Path()] = fact
		}
	}
	return facts
}

// resolveCandidates re-resolves every fact's candidate type names against
// the loaded types.Package for that path.
func resolveCandidates(program *finish.Program, facts map[string]*Fact) []candidate {
	listed := []candidate{}
	for _, path := range slices.Sorted(maps.Keys(facts)) {
		fact := facts[path]
		pkg, found := program.Lookup(path)
		if !found {
			continue
		}
		for _, name := range fact.Types {
			if named, ok := lookupNamed(pkg.Pkg, name); ok {
				listed = append(listed, candidate{pkgPath: path, name: name, named: named})
			}
		}
	}
	sort.Slice(listed, func(i, j int) bool {
		if listed[i].pkgPath != listed[j].pkgPath {
			return listed[i].pkgPath < listed[j].pkgPath
		}
		return listed[i].name < listed[j].name
	})
	return listed
}

// resolveInterfaces re-resolves every fact's interface names against the
// loaded types.Package for that path.
func resolveInterfaces(program *finish.Program, facts map[string]*Fact) []interfaceEntry {
	listed := []interfaceEntry{}
	for _, path := range slices.Sorted(maps.Keys(facts)) {
		fact := facts[path]
		pkg, found := program.Lookup(path)
		if !found {
			continue
		}
		for _, name := range fact.Interfaces {
			obj, ok := pkg.Pkg.Scope().Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			iface, ok := obj.Type().Underlying().(*types.Interface)
			if ok {
				listed = append(listed, interfaceEntry{
					pkgPath: path, name: name, obj: obj, iface: iface,
				})
			}
		}
	}
	sort.Slice(listed, func(i, j int) bool {
		if listed[i].pkgPath != listed[j].pkgPath {
			return listed[i].pkgPath < listed[j].pkgPath
		}
		return listed[i].name < listed[j].name
	})
	return listed
}

// lookupNamed resolves name to its *types.Named object in pkg's scope.
func lookupNamed(pkg *types.Package, name string) (*types.Named, bool) {
	obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
	if !ok {
		return nil, false
	}
	named, ok := obj.Type().(*types.Named)
	return named, ok
}

// implementersOf returns every candidate implementing entry's interface,
// as T or *T. A cross-package candidate is checked against the interface
// as its own package's type-checker saw it, not the driver's independent
// load of the interface's package: packages.Load can produce two distinct
// *types.Package instances for one import path when a test-augmented
// variant exists (dedupe keeps only the augmented one), so a candidate's
// own import graph is walked to find the instance its code actually
// resolved against, falling back to the driver's own view when the
// interface's package is unreachable from there (same-package candidates,
// where the seam cannot arise).
func implementersOf(entry interfaceEntry, candidates []candidate) []candidate {
	matched := []candidate{}
	for _, c := range candidates {
		iface := entry.iface
		if c.pkgPath != entry.pkgPath {
			if seen, ok := interfaceAsSeenBy(c.named.Obj().Pkg(), entry); ok {
				iface = seen
			}
		}
		direct := types.Implements(c.named, iface)
		pointer := types.Implements(types.NewPointer(c.named), iface)
		if direct || pointer {
			matched = append(matched, c)
		}
	}
	return matched
}

// interfaceAsSeenBy looks up the interface in the package at path,
// resolved by walking pkg's own transitive imports rather than the
// driver's independently loaded package set.
func interfaceAsSeenBy(pkg *types.Package, entry interfaceEntry) (*types.Interface, bool) {
	target := findImport(pkg, entry.pkgPath)
	if target == nil {
		return nil, false
	}
	obj, ok := target.Scope().Lookup(entry.name).(*types.TypeName)
	if !ok {
		return nil, false
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	return iface, ok
}

// findImport searches pkg's transitive imports for the package at path,
// pkg itself included: an index-advancing worklist, never recursive.
func findImport(pkg *types.Package, path string) *types.Package {
	seen := map[*types.Package]bool{pkg: true}
	work := []*types.Package{pkg}
	for i := 0; i < len(work); i++ {
		next := work[i]
		if next.Path() == path {
			return next
		}
		for _, imported := range next.Imports() {
			if !seen[imported] {
				seen[imported] = true
				work = append(work, imported)
			}
		}
	}
	return nil
}

// diagnosticFor builds entry's TS-X01 finding, naming impl bare when it
// shares entry's package and package-qualified otherwise.
func diagnosticFor(entry interfaceEntry, impl candidate) analysis.Diagnostic {
	name := impl.name
	if impl.pkgPath != entry.pkgPath {
		name = impl.named.Obj().Pkg().Name() + "." + impl.name
	}
	return analysis.Diagnostic{
		Pos:      entry.obj.Pos(),
		Category: "TS-X01",
		Message: "TS-X01: interface " + entry.name + " has one implementation, " + name +
			" — use " + name + " directly and delete the interface, or add a second " +
			"implementation outside _test.go files",
	}
}
