// Package changed compares the calls each function makes at two revisions and
// reports the calls added and removed. It gets two inventories and compares
// them; it never learns how they were produced.
package changed

import (
	"maps"
	"slices"
	"strings"

	"github.com/kapetan-io/tiger/internal/callinventory"
)

// Sides is the two inventories a report compares.
type Sides struct {
	Base callinventory.Inventory
	Head callinventory.Inventory
}

// Report is the comparison of two inventories. Every difference lands in
// exactly one of Functions, Renamed, NewPackages and RemovedPackages.
type Report struct {
	// Functions is the main list, in byte order of the function header.
	Functions       []Function
	Renamed         []Rename
	NewPackages     []NewPackage
	RemovedPackages []RemovedPackage
	Scope           Scope
}

// Function is a header and the calls added and removed under it, removed
// calls first.
type Function struct {
	Name    string
	Package string
	Owner   string
	Calls   []Call
}

// Call is one added or removed call.
type Call struct {
	Added  bool
	Callee string
	// The fields from Target to Sites exist for the JSON report; the text report
	// prints only Callee. Target is the callee's type or free function, and
	// Method the method called on it, or the function's own name for a free function.
	Target        string
	Method        string
	TargetPackage string
	InModule      bool
	SamePackage   bool
	// Dependency says whether the call changed which packages the function's
	// owner depends on. It is empty for a same-package call.
	Dependency Dependency
	// Sites are where the call is written, at head for an added call and at
	// base for a removed one.
	Sites []callinventory.Site
}

// Dependency is what a call did to the owner's dependency on the callee's package.
type Dependency string

const (
	// DependencyNew is an added call into a package the owner did not call at base.
	DependencyNew Dependency = "new"
	// DependencyDropped is a removed call into a package the owner no longer calls.
	DependencyDropped Dependency = "dropped"
	// DependencyKept is a call into a package the owner calls at both revisions.
	DependencyKept Dependency = "kept"
)

// Rename is an owner that disappeared while one with identical callees
// appeared in the same package.
type Rename struct {
	Package string
	From    string
	To      string
	// Callees is how many types and functions in other packages the new owner calls.
	Callees int
}

// NewPackage is a package that did not exist at base. Its own calls are
// folded under Functions, and Uses says what it is wired to.
type NewPackage struct {
	Name      string
	Owners    int
	Uses      Uses
	Functions []Function
}

// calls counts the added calls in the package.
func (p NewPackage) calls() int {
	return callTotal(p.Functions)
}

// callTotal counts the calls under a list of functions.
func callTotal(functions []Function) int {
	total := 0
	for _, fn := range functions {
		total += len(fn.Calls)
	}
	return total
}

// Uses groups the packages a new package calls into.
type Uses struct {
	Prior   []Use
	New     []Use
	Outside []Use
}

// Use is one package and the types and functions called in it.
type Use struct {
	Package string
	Symbols []string
}

// RemovedPackage is a package that no longer exists at head.
type RemovedPackage struct {
	Name      string
	Owners    int
	Functions []Function
}

// calls counts the removed calls in the package.
func (p RemovedPackage) calls() int {
	return callTotal(p.Functions)
}

// Scope says what was compared and what was left out.
type Scope struct {
	Owners   int
	Packages int
	// Generated lists the generated files skipped at head; base-only ones are not reported.
	Generated   []string
	NotCompared []string
}

// added counts the + lines of the main list.
func (r Report) added() int {
	return r.count(func(call Call) bool { return call.Added })
}

// removed counts the - lines of the main list.
func (r Report) removed() int {
	return r.count(func(call Call) bool { return !call.Added })
}

func (r Report) count(match func(Call) bool) int {
	total := 0
	for _, fn := range r.Functions {
		for _, call := range fn.Calls {
			if match(call) {
				total++
			}
		}
	}
	return total
}

// empty is true when nothing differs in any category.
func (r Report) empty() bool {
	return len(r.Functions) == 0 && len(r.Renamed) == 0 &&
		len(r.NewPackages) == 0 && len(r.RemovedPackages) == 0
}

// owner is a type or free function as one revision has it, with the types and
// functions in other packages it calls.
type owner struct {
	pkg     string
	callees map[string]bool
}

// side is one revision's inventory with its owners worked out.
type side struct {
	inv    callinventory.Inventory
	owners map[string]owner
	// reached is the printed name of every other package each owner calls,
	// initializers included, since a call they make is still a dependency.
	reached map[string]map[string]bool
	// count is how many owners each package has.
	count map[string]int
}

// comparison is the two sides after packages that did not load are removed.
type comparison struct {
	base side
	head side
}

// Compare diffs two inventories. A package that did not load at either
// revision is left out of both, so it never reads as new or removed.
func Compare(sides Sides) Report {
	notCompared := slices.Concat(sides.Base.NotCompared, sides.Head.NotCompared)
	slices.Sort(notCompared)
	notCompared = slices.Compact(notCompared)
	skip := map[string]bool{}
	for _, path := range slices.Concat(sides.Base.Unloaded, sides.Head.Unloaded) {
		skip[path] = true
	}
	roots := rootNames(sides)
	c := comparison{
		base: newSide(qualify(without(sides.Base, skip), roots)),
		head: newSide(qualify(without(sides.Head, skip), roots)),
	}

	newPackages := c.head.onlyHere(c.base)
	gonePackages := c.base.onlyHere(c.head)
	renamed := c.renames()
	renamedOwners := map[string]bool{}
	for _, rename := range renamed {
		renamedOwners[rename.From], renamedOwners[rename.To] = true, true
	}

	report := Report{Renamed: renamed}
	folded := map[string][]Function{}
	sunk := map[string][]Function{}
	for _, name := range c.functionNames() {
		fn := c.function(name)
		if len(fn.Calls) == 0 || renamedOwners[fn.Owner] {
			continue
		}
		if slices.Contains(gonePackages, fn.Package) {
			sunk[fn.Package] = append(sunk[fn.Package], fn)
			continue
		}
		if slices.Contains(newPackages, fn.Package) {
			folded[fn.Package] = append(folded[fn.Package], fn)
			continue
		}
		report.Functions = append(report.Functions, fn)
	}
	for _, pkg := range newPackages {
		report.NewPackages = append(report.NewPackages, NewPackage{
			Name:      pkg,
			Owners:    c.head.count[pkg],
			Uses:      c.usesOf(pkg),
			Functions: folded[pkg],
		})
	}
	for _, pkg := range gonePackages {
		report.RemovedPackages = append(report.RemovedPackages, RemovedPackage{
			Name:      pkg,
			Owners:    c.base.count[pkg],
			Functions: sunk[pkg],
		})
	}
	report.Scope = Scope{
		Owners:      len(c.head.owners),
		Packages:    len(c.head.count),
		Generated:   c.head.inv.Generated,
		NotCompared: notCompared,
	}
	return report
}

// without drops the functions and packages that did not load at either
// revision, identified by import path because printed names can repeat.
func without(inv callinventory.Inventory, skip map[string]bool) callinventory.Inventory {
	kept := callinventory.Inventory{
		Functions: map[string]callinventory.Function{},
		Packages:  map[string]bool{},
		Roots:     map[string]string{},
		Above:     map[string]string{},
		Generated: inv.Generated,
	}
	for _, name := range slices.Sorted(maps.Keys(inv.Functions)) {
		if fn := inv.Functions[name]; !skip[fn.Path] {
			kept.Functions[name] = fn
		}
	}
	for _, name := range slices.Sorted(maps.Keys(inv.Roots)) {
		if !skip[inv.Roots[name]] {
			kept.Roots[name] = inv.Roots[name]
			kept.Packages[name] = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(inv.Above)) {
		if !skip[inv.Above[name]] {
			kept.Above[name] = inv.Above[name]
			kept.Packages[name] = true
		}
	}
	return kept
}

// newSide gathers each owner and the types and functions in other packages it
// calls. An init function is not an owner a reviewer would name.
func newSide(inv callinventory.Inventory) side {
	s := side{inv: inv, owners: map[string]owner{}, reached: map[string]map[string]bool{}, count: map[string]int{}}
	for _, name := range slices.Sorted(maps.Keys(inv.Functions)) {
		fn := inv.Functions[name]
		if s.reached[fn.Owner] == nil {
			s.reached[fn.Owner] = map[string]bool{}
		}
		for _, callee := range slices.Sorted(maps.Keys(fn.Calls)) {
			if call := fn.Calls[callee]; !call.SamePackage {
				s.reached[fn.Owner][call.Package] = true
			}
		}
		if fn.Initializer {
			continue
		}
		entry, ok := s.owners[fn.Owner]
		if !ok {
			entry = owner{pkg: fn.Package, callees: map[string]bool{}}
			s.count[fn.Package]++
		}
		for _, callee := range slices.Sorted(maps.Keys(fn.Calls)) {
			if call := fn.Calls[callee]; !call.SamePackage {
				entry.callees[call.Target] = true
			}
		}
		s.owners[fn.Owner] = entry
	}
	return s
}

// onlyHere lists the packages that have owners on this side and do not exist
// on the other, in byte order. A package of only generated files has no owners.
func (s side) onlyHere(other side) []string {
	var only []string
	for _, pkg := range slices.Sorted(maps.Keys(s.count)) {
		if !other.inv.Packages[pkg] {
			only = append(only, pkg)
		}
	}
	return only
}

// ownersMissingFrom lists the owners of pkg on this side that the other side lacks.
func (s side) ownersMissingFrom(other side, pkg string) []string {
	var missing []string
	for _, name := range slices.Sorted(maps.Keys(s.owners)) {
		if _, ok := other.owners[name]; !ok && s.owners[name].pkg == pkg {
			missing = append(missing, name)
		}
	}
	return missing
}

// renames pairs, per package that exists on both sides, the one owner that
// disappeared with the one that appeared when both call the same set of types
// and functions in other packages.
func (c comparison) renames() []Rename {
	var found []Rename
	for _, pkg := range slices.Sorted(maps.Keys(c.base.inv.Packages)) {
		if !c.head.inv.Packages[pkg] {
			continue
		}
		gone := c.base.ownersMissingFrom(c.head, pkg)
		fresh := c.head.ownersMissingFrom(c.base, pkg)
		if len(gone) != 1 || len(fresh) != 1 {
			continue
		}
		before, after := c.base.owners[gone[0]].callees, c.head.owners[fresh[0]].callees
		if len(before) > 0 && maps.Equal(before, after) {
			found = append(found, Rename{
				Package: pkg, From: gone[0], To: fresh[0], Callees: len(after),
			})
		}
	}
	return found
}

func (c comparison) functionNames() []string {
	names := slices.Concat(
		slices.Sorted(maps.Keys(c.base.inv.Functions)),
		slices.Sorted(maps.Keys(c.head.inv.Functions)),
	)
	slices.Sort(names)
	return slices.Compact(names)
}

// function diffs on call identity first, with both kinds of callee together,
// and only then asks whether a same-package callee counts, using the revision
// the line comes from. Filtering before the diff would print a line under an
// unchanged caller whose callee started or stopped calling out.
func (c comparison) function(name string) Function {
	before, after := c.base.inv.Functions[name], c.head.inv.Functions[name]
	fn := Function{Name: name, Package: after.Package, Owner: after.Owner}
	if _, ok := c.head.inv.Functions[name]; !ok {
		fn.Package, fn.Owner = before.Package, before.Owner
	}
	for _, key := range slices.Sorted(maps.Keys(before.Calls)) {
		if _, kept := after.Calls[key]; !kept && counts(before.Calls[key], c.base.inv) {
			fn.Calls = append(fn.Calls, c.call(before.Calls[key], removal, fn.Owner))
		}
	}
	for _, key := range slices.Sorted(maps.Keys(after.Calls)) {
		if _, was := before.Calls[key]; !was && counts(after.Calls[key], c.head.inv) {
			fn.Calls = append(fn.Calls, c.call(after.Calls[key], addition, fn.Owner))
		}
	}
	slices.SortFunc(fn.Calls, func(a, b Call) int {
		if a.Added != b.Added {
			if a.Added {
				return 1
			}
			return -1
		}
		return strings.Compare(a.Callee, b.Callee)
	})
	return fn
}

// direction says which revision a call exists at.
type direction int

const (
	removal direction = iota
	addition
)

// call builds the line for a call that exists at one revision only. An added
// call reads the owner's packages at base and a removed one reads them at head.
func (c comparison) call(found callinventory.Call, change direction, owner string) Call {
	call := Call{
		Added:         change == addition,
		Callee:        found.Callee,
		Target:        found.Target,
		Method:        found.Method,
		TargetPackage: found.Package,
		InModule:      found.InModule,
		SamePackage:   found.SamePackage,
		Sites:         found.Sites,
	}
	if found.SamePackage {
		return call
	}
	other := c.head
	call.Dependency = DependencyDropped
	if change == addition {
		other = c.base
		call.Dependency = DependencyNew
	}
	if other.reached[owner][found.Package] {
		call.Dependency = DependencyKept
	}
	return call
}

// counts decides whether a call prints. A call into another package always
// does; a call to another owner of the same package does only when that
// function's own body calls another package at this revision.
func counts(call callinventory.Call, inv callinventory.Inventory) bool {
	return !call.SamePackage || inv.Functions[call.Callee].CallsOut
}

// usesOf groups the other packages a new package calls into by whether they
// existed at base.
func (c comparison) usesOf(pkg string) Uses {
	symbols := map[string]map[string]bool{}
	inModule := map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(c.head.inv.Functions)) {
		fn := c.head.inv.Functions[name]
		if fn.Package != pkg {
			continue
		}
		for _, callee := range slices.Sorted(maps.Keys(fn.Calls)) {
			call := fn.Calls[callee]
			if call.SamePackage {
				continue
			}
			if symbols[call.Package] == nil {
				symbols[call.Package] = map[string]bool{}
			}
			symbols[call.Package][call.Symbol] = true
			inModule[call.Package] = call.InModule
		}
	}
	var uses Uses
	for _, target := range slices.Sorted(maps.Keys(symbols)) {
		use := Use{Package: target, Symbols: slices.Sorted(maps.Keys(symbols[target]))}
		switch {
		case !inModule[target]:
			uses.Outside = append(uses.Outside, use)
		case c.base.inv.Packages[target]:
			uses.Prior = append(uses.Prior, use)
		default:
			uses.New = append(uses.New, use)
		}
	}
	return uses
}
