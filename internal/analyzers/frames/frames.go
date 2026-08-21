// Package frames computes TS-F07: every function's frame — the set of
// parameter- and receiver-rooted locations it writes, transitively through
// its callees. A frame answers "which function could have written this
// value": trace the value to a parameter or receiver, and every function
// whose frame contains that location is a candidate; every function whose
// frame does not is provably innocent of writing it.
//
// The frame is computed over the restricted points-to fragment ssalib
// exposes: a store or map update whose target chases through field
// selections, element addresses, and pointer dereferences back to a
// parameter or receiver. Aliasing the chase cannot follow — a pointer
// laundered through a function's return value, a value copied into a
// package global, an interface method call — is silently absent from the
// computed frame, never a finding: TS-P02 bounds precision, and
// correctness constraint 7 forbids a maybe on a blocking rule.
//
// Enforcement applies only to pinned frames, and only on exported functions
// and methods (invariant 3): a computed write outside the pin and a pinned
// location the body never writes both fail, at the pin, bidirectionally
// (constraint 6). Every other function's frame is a computed fact,
// exported for cross-package callers and printed under --show-facts for
// exported unpinned functions.
package frames

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/pins"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/ssalib"
	"github.com/kapetan-io/tiger/internal/directive"
)

// FrameFact is the analyzer's sole exported fact: one exported function's
// frame, in index form so it re-binds through any caller's own parameter
// names. It carries the pinned frame when the function is validly pinned,
// else the traversal-computed one.
type FrameFact struct {
	Locations []ssalib.Location
}

// AFact marks FrameFact as a go/analysis fact.
func (*FrameFact) AFact() {}

// String renders the fact for debugging and for analysistest's fact
// expectation matching: a comma-joined "<param><path>" list, empty for the
// empty frame.
func (f *FrameFact) String() string {
	parts := make([]string, len(f.Locations))
	for i, loc := range f.Locations {
		parts[i] = fmt.Sprintf("%d%s", loc.Param, loc.Path)
	}
	return strings.Join(parts, ",")
}

// Analyzer enforces TS-F07: every function's frame is computed; writes
// outside a pinned frame fail, bidirectionally.
var Analyzer = &analysis.Analyzer{
	Name:      "frames",
	Doc:       "TS-F07: every function's frame is computed; writes outside a pinned frame fail.",
	Run:       run,
	Requires:  []*analysis.Analyzer{buildssa.Analyzer},
	FactTypes: []analysis.Fact{new(FrameFact)},
}

func run(pass *analysis.Pass) (any, error) {
	built, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	assert.Ok(ok, "buildssa result missing")
	funcs := collectFunctions(built.SrcFuncs)
	work := newState(pass, built, funcs)
	work.bindPins(funcDecls(pass))
	work.computeFrames()
	work.reportAndExport()
	return nil, nil
}

// collectFunctions returns every function this package defines, including
// closures: buildssa's SrcFuncs list is built from named declarations, so
// this walks fn.AnonFuncs on a growing worklist — never recursing — to
// pick up every closure transitively nested inside them.
func collectFunctions(seed []*ssa.Function) []*ssa.Function {
	work := append([]*ssa.Function{}, seed...)
	seen := map[*ssa.Function]bool{}
	for _, fn := range work {
		seen[fn] = true
	}
	for i := 0; i < len(work); i++ {
		for _, anon := range work[i].AnonFuncs {
			if seen[anon] {
				continue
			}
			seen[anon] = true
			work = append(work, anon)
		}
	}
	return work
}

// funcDecls returns every top-level function and method declaration in
// pass's files; Go never nests one func declaration inside another, so a
// single pass over each file's top-level Decls finds them all.
func funcDecls(pass *analysis.Pass) []*ast.FuncDecl {
	decls := []*ast.FuncDecl{}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				decls = append(decls, fn)
			}
		}
	}
	return decls
}

// nameSet is a sorted, deduplicated list of rendered location names — what
// a computed frame or a parsed pin reduces to for comparison.
type nameSet []string

// diff returns the entries of set that other does not contain, preserving
// set's order. Enforcement calls it both directions: constraint 6 requires
// checking that every computed write is pinned and every pinned location
// is written.
func (set nameSet) diff(other nameSet) nameSet {
	present := map[string]bool{}
	for _, name := range other {
		present[name] = true
	}
	extra := nameSet{}
	for _, name := range set {
		if !present[name] {
			extra = append(extra, name)
		}
	}
	return extra
}

// computed accumulates one function's traversal-derived frame: the
// location set, in first-seen order, plus the position that introduced
// each one — a direct write or the call that rebased it in from a callee
// — so a violation can name what to fix.
type computed struct {
	seen   map[ssalib.Location]bool
	order  []ssalib.Location
	origin map[ssalib.Location]token.Pos
}

func newComputed() *computed {
	return &computed{
		seen:   map[ssalib.Location]bool{},
		origin: map[ssalib.Location]token.Pos{},
	}
}

// add records loc as introduced at pos the first time it appears; later
// calls for an already-seen location leave its origin untouched. Reports
// whether the set grew, so a fixpoint loop knows another round is needed.
func (c *computed) add(loc ssalib.Location, pos token.Pos) bool {
	if c.seen[loc] {
		return false
	}
	c.seen[loc] = true
	c.order = append(c.order, loc)
	c.origin[loc] = pos
	return true
}

// state carries the per-package bookkeeping one run of the analyzer needs:
// every function's traversal-derived frame, and every pin bound to a
// FuncDecl.
type state struct {
	pass     *analysis.Pass
	prog     *ssa.Program
	pinSet   pins.Set
	funcs    []*ssa.Function
	own      map[*ssa.Function]*computed
	validPin map[*ssa.Function]bool
	hadPin   map[*ssa.Function]bool
	pinLocs  map[*ssa.Function][]ssalib.Location
	pinNames map[*ssa.Function]nameSet
	pinAt    map[*ssa.Function]token.Pos
	declOf   map[*ssa.Function]*ast.FuncDecl
	objOf    map[*ssa.Function]*types.Func
}

func newState(pass *analysis.Pass, built *buildssa.SSA, funcs []*ssa.Function) *state {
	own := map[*ssa.Function]*computed{}
	for _, fn := range funcs {
		own[fn] = newComputed()
	}
	return &state{
		pass:     pass,
		prog:     built.Pkg.Prog,
		pinSet:   pins.Collect(pass.Fset, pass.Files),
		funcs:    funcs,
		own:      own,
		validPin: map[*ssa.Function]bool{},
		hadPin:   map[*ssa.Function]bool{},
		pinLocs:  map[*ssa.Function][]ssalib.Location{},
		pinNames: map[*ssa.Function]nameSet{},
		pinAt:    map[*ssa.Function]token.Pos{},
		declOf:   map[*ssa.Function]*ast.FuncDecl{},
		objOf:    map[*ssa.Function]*types.Func{},
	}
}

// bindPins matches every FuncDecl to its ssa.Function — the same
// TypesInfo.Defs-then-FuncValue lookup buildssa itself uses — and records
// the frame pin bound to it, if any.
func (s *state) bindPins(decls []*ast.FuncDecl) {
	for _, decl := range decls {
		obj, ok := s.pass.TypesInfo.Defs[decl.Name].(*types.Func)
		if !ok {
			continue
		}
		fn := s.prog.FuncValue(obj)
		if fn == nil {
			continue
		}
		s.declOf[fn] = decl
		s.objOf[fn] = obj
		s.bindOne(fn, decl)
	}
}

// bindOne records the single frame pin bound to decl, if it is well-formed
// and exported. More than one pin is ambiguous, and any pin on an
// unexported function violates invariant 3 — both blocking, both reported
// here rather than deferred to enforcement, and both leave fn unpinned for
// frame computation.
func (s *state) bindOne(fn *ssa.Function, decl *ast.FuncDecl) {
	matched := s.pinSet.At(decl, "frame")
	if len(matched) == 0 {
		return
	}
	s.hadPin[fn] = true
	if len(matched) > 1 {
		s.reportAmbiguous(decl, matched)
		return
	}
	pin := matched[0]
	if !decl.Name.IsExported() {
		s.reportUnexportedPin(decl, pin)
		return
	}
	names, err := directive.ParseFrame(pin.Directive.Args)
	assert.Ok(err == nil, "a pin collected by pins.Collect re-parses cleanly")
	s.validPin[fn] = true
	s.pinNames[fn] = nameSet(names)
	s.pinAt[fn] = pin.Pos
	s.pinLocs[fn] = paramLocations(fn, names)
}

// reportAmbiguous fires TS-F07 when decl carries more than one frame pin.
func (s *state) reportAmbiguous(decl *ast.FuncDecl, matched []pins.Pin) {
	s.pass.Report(analysis.Diagnostic{
		Pos:      matched[0].Pos,
		Category: "TS-F07",
		Message: fmt.Sprintf(
			"TS-F07: %s has %d frame pins — keep exactly one", decl.Name.Name, len(matched)),
	})
}

// reportUnexportedPin fires TS-F07 when decl's frame pin sits on an
// unexported function or method — invariant 3 binds pins to exported
// declarations only.
func (s *state) reportUnexportedPin(decl *ast.FuncDecl, pin pins.Pin) {
	s.pass.Report(analysis.Diagnostic{
		Pos:      pin.Pos,
		Category: "TS-F07",
		Message: fmt.Sprintf(
			"TS-F07: frame pin on unexported function %s — a pin binds only exported "+
				"functions and methods; remove the pin or export %s",
			decl.Name.Name, decl.Name.Name),
	})
}

// paramLocations converts a pin's rendered location names back to index
// form against fn's own parameter names — the inverse of
// ssalib.RenderLocation, needed so a pinned function can act as a modular
// summary for its callers. A name whose root does not match any parameter
// is dropped: the pin was validated syntactically, not against a
// signature.
func paramLocations(fn *ssa.Function, names []string) []ssalib.Location {
	locs := []ssalib.Location{}
	for _, name := range names {
		loc, ok := paramLocation(fn, name)
		if ok {
			locs = append(locs, loc)
		}
	}
	return locs
}

// paramLocation resolves one rendered name ("r.log") to the parameter it
// roots at and the path beneath it.
func paramLocation(fn *ssa.Function, name string) (ssalib.Location, bool) {
	root, path := name, ""
	if cut := strings.IndexByte(name, '.'); cut >= 0 {
		root, path = name[:cut], name[cut:]
	}
	for i, param := range fn.Params {
		if param.Name() == root {
			return ssalib.Location{Param: i, Path: path}, true
		}
	}
	return ssalib.Location{}, false
}

// computeFrames runs the traversal fixpoint: same-package strongly
// connected components in reverse topological order, each solved by a
// bounded round loop imitating ssalib's own SCC walk. A validly pinned
// function's own traversal is still computed here — enforcement needs it
// — only its use as a callee's summary is short-circuited to the pin.
func (s *state) computeFrames() {
	comps := ssalib.Components(s.funcs, s.callees)
	for _, comp := range comps {
		s.runComponent(comp)
	}
}

// callees lists fn's statically resolvable same-package calls, dropping
// edges into validly pinned functions: their frame is fixed from the pin
// before any fixpoint round runs, so they need no place in the dependency
// order Components computes.
func (s *state) callees(fn *ssa.Function) []*ssa.Function {
	listed := []*ssa.Function{}
	for _, call := range ssalib.StaticCalls(fn) {
		if s.validPin[call.Callee] {
			continue
		}
		listed = append(listed, call.Callee)
	}
	return listed
}

// runComponent solves one strongly connected component with a capped round
// loop: every round recomputes every member's frame from scratch, and the
// loop stops the round it adds nothing. A new location can propagate at
// most one hop around the cycle per round, so a component of n members
// converges within n rounds; the +2 margin and the assert turn a
// bookkeeping bug into a loud failure instead of an infinite loop.
func (s *state) runComponent(comp []*ssa.Function) {
	rounds := len(comp) + 2
	stable := false
	for round := 0; round < rounds; round++ {
		changed := false
		for _, fn := range comp {
			if s.updateOwn(fn) {
				changed = true
			}
		}
		if !changed {
			stable = true
			break
		}
	}
	assert.Ok(stable, "frame fixpoint did not converge within its round budget")
}

// updateOwn recomputes fn's own frame from its writes and its calls'
// rebased callee frames, reporting whether the set grew.
func (s *state) updateOwn(fn *ssa.Function) bool {
	own := s.own[fn]
	changed := false
	for _, write := range ssalib.Writes(fn) {
		if own.add(write.Loc, write.Pos) {
			changed = true
		}
	}
	for _, call := range ssalib.StaticCalls(fn) {
		if s.rebaseCall(own, call) {
			changed = true
		}
	}
	return changed
}

// rebaseCall folds one call's callee frame into own, rebased through the
// call's arguments (ssalib.RebaseArg; an argument shape it cannot rebase
// is a known-miss, dropped). Reports whether it added anything.
func (s *state) rebaseCall(own *computed, call ssalib.StaticCall) bool {
	callee, ok := s.calleeFrame(call.Callee)
	if !ok {
		return false
	}
	changed := false
	for _, loc := range callee {
		rebased, ok := ssalib.RebaseArg(call.Args, loc)
		if !ok {
			continue
		}
		if own.add(rebased, call.Pos) {
			changed = true
		}
	}
	return changed
}

// calleeFrame resolves callee's frame as its caller should see it: the pin
// when callee is validly pinned, the in-progress same-package traversal
// when callee is a member of this package (named or a closure), or the
// cross-package fact when callee is external. A callee none of these
// resolve — most often an unnamed external function value — is a
// known-miss: absent, never a finding.
func (s *state) calleeFrame(callee *ssa.Function) ([]ssalib.Location, bool) {
	if s.validPin[callee] {
		return s.pinLocs[callee], true
	}
	if own, ok := s.own[callee]; ok {
		return own.order, true
	}
	obj := callee.Object()
	if obj == nil {
		return nil, false
	}
	var fact FrameFact
	if !s.pass.ImportObjectFact(obj, &fact) {
		return nil, false
	}
	return fact.Locations, true
}

// reportAndExport runs enforcement and fact reporting for every named
// function, then exports a FrameFact for every exported one. Export runs
// last so every exported fact reflects a fully converged frame. Closures
// are skipped entirely — only named declarations carry pins, findings, or
// facts — and so are unexported functions: Go cannot import an unexported
// symbol, so no other package's pass could ever resolve a fact on one
// through ImportObjectFact, and this package's own callers already read
// its frame straight out of the in-memory traversal, never through a fact.
func (s *state) reportAndExport() {
	for _, fn := range s.funcs {
		decl, named := s.declOf[fn]
		if !named {
			continue
		}
		switch {
		case s.validPin[fn]:
			s.enforcePin(fn)
		case !s.hadPin[fn] && decl.Name.IsExported():
			s.reportFacts(fn, decl)
		}
		if decl.Name.IsExported() {
			s.export(fn)
		}
	}
}

// enforcePin compares fn's own computed frame against its pin, both
// rendered to names, as sets in both directions (constraint 6): a
// computed write the pin does not list, and a pinned location the body
// never writes.
func (s *state) enforcePin(fn *ssa.Function) {
	rendered := renderSorted(fn, s.own[fn].order)
	origin := originByName(fn, s.own[fn])
	pinAt := s.pinAt[fn]
	fix := directive.Format(directive.Directive{
		Verb: "frame", Args: directive.FormatFrame(rendered),
	})
	for _, name := range rendered.diff(s.pinNames[fn]) {
		s.pass.Report(analysis.Diagnostic{
			Pos:      pinAt,
			Category: "TS-F07",
			Message: fmt.Sprintf(
				"TS-F07: computed frame writes %s, introduced at %s, outside the pinned "+
					"frame — remove the write or update the pin to %s",
				name, nearPos(s.pass.Fset, origin[name]), fix),
		})
	}
	for _, name := range s.pinNames[fn].diff(rendered) {
		s.pass.Report(analysis.Diagnostic{
			Pos:      pinAt,
			Category: "TS-F07",
			Message: fmt.Sprintf(
				"TS-F07: pinned frame location %s is never written — tighten the pin to %s",
				name, fix),
		})
	}
}

// reportFacts prints fn's computed frame as a TS-F07-facts finding, in
// exact pin syntax: an unpinned exported function's frame is a fact, never
// a violation.
func (s *state) reportFacts(fn *ssa.Function, decl *ast.FuncDecl) {
	rendered := renderSorted(fn, s.own[fn].order)
	s.pass.Report(analysis.Diagnostic{
		Pos:      decl.Pos(),
		Category: "TS-F07-facts",
		Message: "TS-F07: computed frame for " + decl.Name.Name + " — " + directive.Format(
			directive.Directive{Verb: "frame", Args: directive.FormatFrame(rendered)}),
	})
}

// export publishes fn's FrameFact: the pin's index-form locations when fn
// is validly pinned, else the traversal-computed ones — the summary every
// caller, same package or cross-package, resolves through calleeFrame.
func (s *state) export(fn *ssa.Function) {
	locs := s.own[fn].order
	if s.validPin[fn] {
		locs = s.pinLocs[fn]
	}
	s.pass.ExportObjectFact(s.objOf[fn], &FrameFact{Locations: locs})
}

// renderSorted renders locs through fn's own parameter names, dropping
// anything RenderLocation cannot name, and sorts the result — the
// determinism every comparison and every printed fact needs.
func renderSorted(fn *ssa.Function, locs []ssalib.Location) nameSet {
	names := nameSet{}
	for _, loc := range locs {
		name, ok := ssalib.RenderLocation(fn, loc)
		if ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// originByName maps each of fn's computed locations to its introducing
// position, keyed by the same rendered name enforcePin compares against
// the pin.
func originByName(fn *ssa.Function, own *computed) map[string]token.Pos {
	byName := map[string]token.Pos{}
	for _, loc := range own.order {
		name, ok := ssalib.RenderLocation(fn, loc)
		if ok {
			byName[name] = own.origin[loc]
		}
	}
	return byName
}

// nearPos prints a position with only the file's base name: the driver
// relativizes finding positions but never message text, so an absolute
// path here would reach the output and break cross-machine determinism.
func nearPos(fset *token.FileSet, pos token.Pos) string {
	at := fset.Position(pos)
	return fmt.Sprintf("%s:%d:%d", filepath.Base(at.Filename), at.Line, at.Column)
}
