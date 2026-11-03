// Package changed is tiger changed: it compares the calls each function makes
// at two revisions and reports the calls added and removed. Run loads the two
// inventories; Compare gets them and never learns how they were produced.
package changed

import (
	"maps"
	"slices"
	"strings"

	"github.com/kapetan-io/tiger/internal/callinventory"
)

type Sides struct {
	Base callinventory.Inventory
	Head callinventory.Inventory
}

type Report struct {
	Functions       []Function
	Renamed         []Rename
	NewPackages     []NewPackage
	RemovedPackages []RemovedPackage
	Scope           Scope
}

type Function struct {
	Name    string
	Package string
	Owner   string
	Calls   []Call
}

type Call struct {
	Added         bool
	Callee        string
	Target        string
	Method        string
	TargetPackage string
	InModule      bool
	SamePackage   bool
	Dependency    Dependency
	Sites         []callinventory.Site
}

type Dependency string

const (
	DependencyNew     Dependency = "new"
	DependencyDropped Dependency = "dropped"
	DependencyKept    Dependency = "kept"
)

type Rename struct {
	Package string
	From    string
	To      string
	Callees int
}

type NewPackage struct {
	Name      string
	Owners    int
	Uses      Uses
	Functions []Function
}

func (p NewPackage) calls() int {
	return callTotal(p.Functions)
}

func callTotal(functions []Function) int {
	total := 0
	for _, fn := range functions {
		total += len(fn.Calls)
	}
	return total
}

type Uses struct {
	Prior   []Use
	New     []Use
	Outside []Use
}

type Use struct {
	Package string
	Symbols []string
}

type RemovedPackage struct {
	Name      string
	Owners    int
	Functions []Function
}

func (p RemovedPackage) calls() int {
	return callTotal(p.Functions)
}

type Scope struct {
	Owners      int
	Packages    int
	Generated   []string
	NotCompared []string
}

func (r Report) added() int {
	return r.count(func(call Call) bool { return call.Added })
}

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

func (r Report) empty() bool {
	return len(r.Functions) == 0 && len(r.Renamed) == 0 &&
		len(r.NewPackages) == 0 && len(r.RemovedPackages) == 0
}

type owner struct {
	pkg     string
	callees map[string]bool
}

type side struct {
	inv     callinventory.Inventory
	owners  map[string]owner
	reached map[string]map[string]bool
	count   map[string]int
}

type comparison struct {
	base side
	head side
}

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

func newSide(inv callinventory.Inventory) side {
	s := side{
		inv: inv, owners: map[string]owner{}, count: map[string]int{},
		reached: map[string]map[string]bool{},
	}
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

func (s side) onlyHere(other side) []string {
	var only []string
	for _, pkg := range slices.Sorted(maps.Keys(s.count)) {
		if !other.inv.Packages[pkg] {
			only = append(only, pkg)
		}
	}
	return only
}

func (s side) ownersMissingFrom(other side, pkg string) []string {
	var missing []string
	for _, name := range slices.Sorted(maps.Keys(s.owners)) {
		if _, ok := other.owners[name]; !ok && s.owners[name].pkg == pkg {
			missing = append(missing, name)
		}
	}
	return missing
}

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

type direction int

const (
	removal direction = iota
	addition
)

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

func counts(call callinventory.Call, inv callinventory.Inventory) bool {
	return !call.SamePackage || inv.Functions[call.Callee].CallsOut
}

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
