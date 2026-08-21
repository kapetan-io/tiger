// Package effects enforces TS-F01 and TS-F02: every function's effect set
// is computed, and a pin freezes it into a bidirectional contract over the
// function's whole subtree.
//
// The fact lifecycle is computed → reported → pinned. Every function's
// effect set is computed unconditionally — its own instructions, its
// static calls, the stdlib effects table — with no annotation required.
// An unpinned exported function's computed set is printed as a TS-F01-facts
// diagnostic (findings.Reported severity, visible only under --show-facts):
// this is the "reported" stage, exact pin syntax so a later wave (ENG-151,
// tiger pin) can freeze it by pasting it as-is. A pin turns a reported fact
// into a contract: TS-F01 compares the pin against the computed set in
// both directions, because a pin that only catches widening and never
// catches narrowing is a defensive superset that stops meaning anything.
// TS-F02 is what makes a pin worth writing at all — it bounds the pinned
// function's entire subtree, so a same-package unexported helper or a call
// three frames into another package cannot silently launder an effect the
// pin does not declare; the finding fires at the pin, naming the call that
// introduced the widening.
//
// Effect sets cross package boundaries through go/analysis object facts
// (EffectsFact): a pinned function exports its pin as the fact content,
// an unpinned one exports its computed set, and a caller in another
// package imports the fact instead of re-deriving the callee's body —
// which is also why a pinned callee's own subtree never needs revisiting
// from the caller's side (TS-F02's modularity).
//
// Two composition paths are structurally undecidable and resolve to a
// known-miss instead of a finding: a call through a value that is not
// statically known (StaticCalls only resolves direct calls, method calls
// on concrete types, and function literals with a known target), and a
// callee outside this module with no stdlib table entry and no fact to
// import. Both under-approximate on purpose — a missed effect is a gap in
// this wave's coverage, never a false positive on a blocking rule.
package effects

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

// EffectsFact carries one named function's enforced effect summary in
// index form across the facts mechanism. Set carries the bare atoms and
// IO qualifiers only — its Mutate field is unused, because a name only
// means something rebound to the importing function's own parameters,
// which happens at render time, never inside the fact itself. Mutate
// carries that same information as index-rooted Locations instead.
type EffectsFact struct {
	Set    directive.EffectSet
	Mutate []ssalib.Location
}

// AFact marks EffectsFact as a go/analysis fact.
func (f *EffectsFact) AFact() {}

// String renders a fact in the same pin-syntax effect text FormatEffects
// prints, so analysistest's fact expectations and any -debug=t output
// read the same vocabulary as everything else this analyzer prints.
func (f *EffectsFact) String() string {
	return directive.FormatEffects(f.Set)
}

// Analyzer enforces TS-F01 and TS-F02 and reports computed facts under
// Category "TS-F01-facts".
var Analyzer = &analysis.Analyzer{
	Name: "effects",
	Doc: "TS-F01 and TS-F02: every function's effect set is computed; a pin makes it a " +
		"bidirectional contract over its whole subtree.",
	Requires:  []*analysis.Analyzer{buildssa.Analyzer},
	FactTypes: []analysis.Fact{new(EffectsFact)},
	Run:       run,
}

// funcEntry is one declared function or method: its AST, its type object,
// its SSA form, and whatever effects pins bind to its declaration.
type funcEntry struct {
	decl       *ast.FuncDecl
	obj        *types.Func
	fn         *ssa.Function
	effectPins []pins.Pin
}

// summary is one function's contribution to a caller: bare atoms and IO
// (Mutate left empty) plus mutate locations in the contributing
// function's own parameter index frame.
type summary struct {
	Set    directive.EffectSet
	Mutate []ssalib.Location
}

// sourceKind distinguishes an atom's provenance for enforcement messages:
// the function's own instructions, or a call that introduced it.
type sourceKind string

const (
	sourceInstruction sourceKind = "instruction"
	sourceCall        sourceKind = "call"
)

// source locates the first instruction or call that introduced one atom
// or mutate location, for naming in a TS-F01/TS-F02 message.
type source struct {
	Pos    token.Pos
	Detail string
	Kind   sourceKind
}

// accum is one function's own composed contribution: its value (embedded
// in summary) plus first-origin provenance per atom and mutate location,
// keyed by atomsPresent's term or mutateKey's location key.
type accum struct {
	summary
	Origins map[string]source
}

// equal reports whether two composed contributions carry the same value —
// the fixpoint's stability check. Only value fields matter; provenance
// never changes what the value is.
func (a accum) equal(b accum) bool {
	return a.Set.Equal(b.Set) && slices.Equal(a.Mutate, b.Mutate)
}

func run(pass *analysis.Pass) (any, error) {
	built, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	assert.Ok(ok, "buildssa result missing")
	collected := pins.Collect(pass.Fset, pass.Files)

	allFns := gatherFunctions(built.SrcFuncs)
	ssaOf := map[*types.Func]*ssa.Function{}
	for _, fn := range allFns {
		if obj, ok := fn.Object().(*types.Func); ok {
			ssaOf[obj] = fn
		}
	}

	entries := buildEntries(pass, collected, ssaOf)
	pinnedFns := buildPinned(pass, entries)

	computed := map[*ssa.Function]accum{}
	for _, comp := range ssalib.Components(unpinned(allFns, pinnedFns), calleesOf) {
		runComponent(pass, comp, computed, pinnedFns)
	}

	exportFacts(pass, entries, computed, pinnedFns)
	reportFacts(pass, entries, computed)
	reportPins(pass, entries, computed, pinnedFns)

	return nil, nil
}

// gatherFunctions collects every function in the package's SSA form,
// declared and closure alike: an index-advancing worklist over
// AnonFuncs, never recursive.
func gatherFunctions(roots []*ssa.Function) []*ssa.Function {
	all := []*ssa.Function{}
	seen := map[*ssa.Function]bool{}
	work := append([]*ssa.Function{}, roots...)
	for i := 0; i < len(work); i++ {
		fn := work[i]
		if seen[fn] {
			continue
		}
		seen[fn] = true
		all = append(all, fn)
		work = append(work, fn.AnonFuncs...)
	}
	return all
}

// buildEntries walks the package's declarations in source order, pairing
// each with its type object, its SSA form, and its effects pins.
func buildEntries(
	pass *analysis.Pass, collected pins.Set, ssaOf map[*types.Func]*ssa.Function,
) []funcEntry {
	entries := []funcEntry{}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			obj, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func)
			if !ok {
				continue
			}
			fn, ok := ssaOf[obj]
			if !ok {
				continue
			}
			entries = append(entries, funcEntry{
				decl:       fd,
				obj:        obj,
				fn:         fn,
				effectPins: collected.At(fd, "effects"),
			})
		}
	}
	return entries
}

// buildPinned validates each entry's effects pins and returns the
// modular summary for every validly pinned (single pin, exported name)
// function — TS-F02's "callee pinned" case. An ambiguous or
// unexported-target pin is reported here (TS-F01, invariant 3) and its
// function is left out, so composition falls back to its real computed
// effects rather than an invalid contract.
func buildPinned(pass *analysis.Pass, entries []funcEntry) map[*ssa.Function]summary {
	pinnedFns := map[*ssa.Function]summary{}
	for _, e := range entries {
		if len(e.effectPins) == 0 {
			continue
		}
		if len(e.effectPins) > 1 {
			pass.Report(analysis.Diagnostic{
				Pos:      e.decl.Pos(),
				Category: "TS-F01",
				Message: "TS-F01: more than one effects pin on this function is an " +
					"ambiguous contract — keep exactly one",
			})
			continue
		}
		pin := e.effectPins[0]
		if !ast.IsExported(e.decl.Name.Name) {
			pass.Report(analysis.Diagnostic{
				Pos:      pin.Pos,
				Category: "TS-F01",
				Message: "TS-F01: a pin may only appear on an exported function or " +
					"method — remove this pin; the exported pins above already " +
					"constrain this helper (TS-F02)",
			})
			continue
		}
		declared, err := directive.ParseEffects(pin.Directive.Args)
		assert.NoError(err, "effects pin reparse")
		mutateNames := declared.Mutate
		declared.Mutate = nil
		pinnedFns[e.fn] = summary{Set: declared, Mutate: convertMutateNames(e.fn, mutateNames)}
	}
	return pinnedFns
}

// unpinned returns allFns minus the functions with a valid exported pin —
// TS-F02's "do not traverse beneath" a pinned callee, expressed as the
// node set the fixpoint composition ever needs to visit.
func unpinned(allFns []*ssa.Function, pinnedFns map[*ssa.Function]summary) []*ssa.Function {
	kept := []*ssa.Function{}
	for _, fn := range allFns {
		if _, ok := pinnedFns[fn]; !ok {
			kept = append(kept, fn)
		}
	}
	return kept
}

// calleesOf lists fn's statically resolvable callees, for the SCC walk
// that orders same-package composition.
func calleesOf(fn *ssa.Function) []*ssa.Function {
	callees := []*ssa.Function{}
	for _, call := range ssalib.StaticCalls(fn) {
		callees = append(callees, call.Callee)
	}
	return callees
}

// runComponent resolves one strongly-connected component to a fixpoint: a
// bounded round loop re-composes every member until nothing changes. The
// bound is generous because the round count needed is at most the number
// of distinct atoms and mutate locations reachable in the component, not
// its size — exhaustion is a bookkeeping bug, not a valid outcome.
func runComponent(
	pass *analysis.Pass, comp []*ssa.Function, computed map[*ssa.Function]accum,
	pinnedFns map[*ssa.Function]summary,
) {
	for _, fn := range comp {
		computed[fn] = accum{Origins: map[string]source{}}
	}
	roundCap := len(comp) + 16
	for round := 0; round < roundCap; round++ {
		changed := false
		for _, fn := range comp {
			next := compose(pass, fn, computed, pinnedFns)
			if !computed[fn].equal(next) {
				computed[fn] = next
				changed = true
			}
		}
		if !changed {
			return
		}
	}
	assert.Fail("effects fixpoint over %d functions did not converge", len(comp))
}

// compose computes fn's own composed contribution: its own instructions
// first, then each static call's contribution in call order. A pinned
// same-package callee contributes its pin (TS-F02 modularity, never
// traversed beneath); an unpinned same-package callee contributes its
// resolved value; a stdlib or cross-package callee contributes the table
// entry or the imported fact. A callee the composition cannot resolve —
// an interface call, an unresolved function value, a third-party callee
// with no fact — contributes nothing, a known-miss.
func compose(
	pass *analysis.Pass, fn *ssa.Function, computed map[*ssa.Function]accum,
	pinnedFns map[*ssa.Function]summary,
) accum {
	found := accum{Origins: map[string]source{}}

	own := ssalib.InstructionEffects(fn)
	mergeOwnAtoms(&found, own)
	mergeOwnMutate(&found, fn, ssalib.Writes(fn))

	for _, call := range ssalib.StaticCalls(fn) {
		resolved := resolveCall(pass, call, computed, pinnedFns)
		detail := call.Callee.String()
		mergeCallAtoms(&found, resolved.Set, call.Pos, detail)
		mergeCallMutate(&found, rebaseAll(call.Args, resolved.Mutate), call.Pos, detail)
	}
	return found
}

// resolveCall resolves one static call to its contribution, in the
// composition order TS-F02 specifies: a pinned same-package callee's pin,
// an unpinned same-package callee's resolved value, the stdlib table,
// then an imported cross-package fact. Nothing resolvable contributes the
// zero summary.
func resolveCall(
	pass *analysis.Pass, call ssalib.StaticCall, computed map[*ssa.Function]accum,
	pinnedFns map[*ssa.Function]summary,
) summary {
	if found, ok := pinnedFns[call.Callee]; ok {
		return found
	}
	if found, ok := computed[call.Callee]; ok {
		return found.summary
	}
	if found, ok := ssalib.Stdlib(call.Callee); ok {
		return summary{Set: found}
	}
	if obj := call.Callee.Object(); obj != nil {
		var fact EffectsFact
		if pass.ImportObjectFact(obj, &fact) {
			return summary(fact)
		}
	}
	return summary{}
}

// rebaseAll rebases every callee-frame location through one call's
// arguments into the caller's frame, dropping the ones RebaseArg cannot
// map — a caller-local mutation or an alias, a known-miss.
func rebaseAll(args []ssa.Value, locs []ssalib.Location) []ssalib.Location {
	rebased := []ssalib.Location{}
	for _, loc := range locs {
		if next, ok := ssalib.RebaseArg(args, loc); ok {
			rebased = append(rebased, next)
		}
	}
	return rebased
}

// mergeOwnAtoms folds fn's own-instruction atoms into found, recording an
// instruction-sourced origin the first time each atom appears.
func mergeOwnAtoms(found *accum, own ssalib.Contribution) {
	for _, key := range atomsPresent(own.Set) {
		if !hasAtomKey(found.Set, key) {
			hit := own.Origins[key]
			found.Origins[key] = source{Pos: hit.Pos, Detail: hit.Detail, Kind: sourceInstruction}
		}
	}
	found.Set = found.Set.Union(own.Set)
}

// mergeCallAtoms folds one call's contributed atoms into found, recording
// a call-sourced origin the first time each atom appears.
func mergeCallAtoms(found *accum, add directive.EffectSet, pos token.Pos, detail string) {
	for _, key := range atomsPresent(add) {
		if !hasAtomKey(found.Set, key) {
			found.Origins[key] = source{Pos: pos, Detail: detail, Kind: sourceCall}
		}
	}
	found.Set = found.Set.Union(add)
}

// mergeOwnMutate folds fn's own parameter-rooted writes into found,
// recording an instruction-sourced origin for each new location.
func mergeOwnMutate(found *accum, fn *ssa.Function, writes []ssalib.Write) {
	for _, write := range writes {
		if containsLocation(found.Mutate, write.Loc) {
			continue
		}
		found.Mutate = append(found.Mutate, write.Loc)
		name, _ := ssalib.RenderLocation(fn, write.Loc)
		found.Origins[mutateKey(write.Loc)] = source{
			Pos: write.Pos, Detail: "write to " + name, Kind: sourceInstruction,
		}
	}
}

// mergeCallMutate folds one call's rebased mutate locations into found,
// recording a call-sourced origin for each new location.
func mergeCallMutate(found *accum, locs []ssalib.Location, pos token.Pos, detail string) {
	for _, loc := range locs {
		if containsLocation(found.Mutate, loc) {
			continue
		}
		found.Mutate = append(found.Mutate, loc)
		found.Origins[mutateKey(loc)] = source{Pos: pos, Detail: detail, Kind: sourceCall}
	}
}

// containsLocation reports whether locs already holds loc.
func containsLocation(locs []ssalib.Location, loc ssalib.Location) bool {
	for _, existing := range locs {
		if existing == loc {
			return true
		}
	}
	return false
}

// mutateKey renders loc as an Origins map key, independent of any
// function's own parameter names.
func mutateKey(loc ssalib.Location) string {
	return fmt.Sprintf("mutate:%d:%s", loc.Param, loc.Path)
}

// atomsPresent lists set's bare atoms and IO qualifiers in the lattice's
// canonical order — never a map range, so output stays deterministic.
func atomsPresent(set directive.EffectSet) []string {
	present := []string{}
	if set.Alloc {
		present = append(present, "alloc")
	}
	for _, qualifier := range set.IO {
		present = append(present, "io("+qualifier+")")
	}
	if set.Block {
		present = append(present, "block")
	}
	if set.Panic {
		present = append(present, "panic")
	}
	if set.Rand {
		present = append(present, "rand")
	}
	if set.Time {
		present = append(present, "time")
	}
	if set.Spawn {
		present = append(present, "spawn")
	}
	return present
}

// hasAtomKey reports whether set already carries the atom or IO qualifier
// atomsPresent would render as key.
func hasAtomKey(set directive.EffectSet, key string) bool {
	for _, present := range atomsPresent(set) {
		if present == key {
			return true
		}
	}
	return false
}

// convertMutateNames converts a pin's name-form mutate paths back to
// index form through fn's own parameters — the reverse of RenderLocation.
func convertMutateNames(fn *ssa.Function, names []string) []ssalib.Location {
	locs := []ssalib.Location{}
	for _, name := range names {
		if loc, ok := parseMutateName(fn, name); ok {
			locs = append(locs, loc)
		}
	}
	return locs
}

// parseMutateName resolves one dotted name ("r.log") to the parameter it
// roots through, by matching fn's own parameter names.
func parseMutateName(fn *ssa.Function, name string) (ssalib.Location, bool) {
	root, rest, hasPath := strings.Cut(name, ".")
	for i, param := range fn.Params {
		if param.Name() != root {
			continue
		}
		if !hasPath {
			return ssalib.Location{Param: i}, true
		}
		return ssalib.Location{Param: i, Path: "." + rest}, true
	}
	return ssalib.Location{}, false
}

// renderEffectSet renders val through fn's own parameter names into the
// full EffectSet pin syntax compares against — unrenderable mutate
// locations are dropped from the comparison, a known-miss.
func renderEffectSet(fn *ssa.Function, val accum) directive.EffectSet {
	set := val.Set
	names := []string{}
	for _, loc := range val.Mutate {
		if name, ok := ssalib.RenderLocation(fn, loc); ok {
			names = append(names, name)
		}
	}
	set.Mutate = names
	return set
}

// exportFacts exports one EffectsFact per exported named function: the
// pin when pinned, the computed value otherwise. Facts make TS-F02 cross
// a package boundary — a caller in another package imports this instead
// of re-deriving the callee's body. Unexported functions are never
// callable from another package, so a fact for one could never be
// imported; exporting only the exported surface keeps the fact set
// exactly as large as TS-F02's cross-package path can ever consume.
func exportFacts(
	pass *analysis.Pass, entries []funcEntry, computed map[*ssa.Function]accum,
	pinnedFns map[*ssa.Function]summary,
) {
	for _, e := range entries {
		if !ast.IsExported(e.decl.Name.Name) {
			continue
		}
		if found, ok := pinnedFns[e.fn]; ok {
			pass.ExportObjectFact(e.obj, &EffectsFact{Set: found.Set, Mutate: found.Mutate})
			continue
		}
		found := computed[e.fn]
		pass.ExportObjectFact(e.obj, &EffectsFact{Set: found.Set, Mutate: found.Mutate})
	}
}

// reportFacts prints the computed set for every exported, unpinned
// function as a TS-F01-facts diagnostic, in exact pin syntax — the
// "reported" stage of the fact lifecycle.
func reportFacts(pass *analysis.Pass, entries []funcEntry, computed map[*ssa.Function]accum) {
	for _, e := range entries {
		if len(e.effectPins) != 0 {
			continue
		}
		if !ast.IsExported(e.decl.Name.Name) {
			continue
		}
		rendered := renderEffectSet(e.fn, computed[e.fn])
		pass.Report(analysis.Diagnostic{
			Pos:      e.decl.Pos(),
			Category: "TS-F01-facts",
			Message: fmt.Sprintf("TS-F01: computed effects for %s — %s",
				e.decl.Name.Name,
				directive.Format(directive.Directive{
					Verb: "effects", Args: directive.FormatEffects(rendered),
				})),
		})
	}
}

// reportPins enforces every validly pinned function's contract in both
// directions.
func reportPins(
	pass *analysis.Pass, entries []funcEntry, computed map[*ssa.Function]accum,
	pinnedFns map[*ssa.Function]summary,
) {
	for _, e := range entries {
		if len(e.effectPins) != 1 {
			continue
		}
		if _, ok := pinnedFns[e.fn]; !ok {
			continue
		}
		val := compose(pass, e.fn, computed, pinnedFns)
		reportEnforcement(pass, e.fn, e.effectPins[0], val)
	}
}

// reportEnforcement compares fn's computed set against its pin in both
// directions (constraint 6): a declared-but-absent effect is always a
// finding, and an undeclared computed effect is a finding naming its
// introducing instruction or call.
func reportEnforcement(pass *analysis.Pass, fn *ssa.Function, pin pins.Pin, val accum) {
	declared, err := directive.ParseEffects(pin.Directive.Args)
	assert.NoError(err, "effects pin reparse")
	rendered := renderEffectSet(fn, val)

	missing := declared.Diff(rendered)
	if !missing.Empty() {
		pass.Report(analysis.Diagnostic{
			Pos:      pin.Pos,
			Category: "TS-F01",
			Message: fmt.Sprintf(
				"TS-F01: pin declares %s that the computed effects do not have — "+
					"tighten the pin to %s",
				directive.FormatEffects(missing),
				directive.Format(directive.Directive{
					Verb: "effects", Args: directive.FormatEffects(rendered),
				}),
			),
		})
	}

	widened := rendered.Diff(declared)
	if !widened.Empty() {
		reportWidening(pass, pin, violation{Fn: fn, Val: val, Widened: widened, Rendered: rendered})
	}
}

// violation bundles one undeclared-widening finding's inputs — fn and val
// to trace the representative atom's origin, Widened and Rendered to
// build the message — under reportPins' four-parameter budget.
type violation struct {
	Fn       *ssa.Function
	Val      accum
	Widened  directive.EffectSet
	Rendered directive.EffectSet
}

// reportWidening reports the undeclared-effects direction: TS-F02 when
// the representative widened atom arrived through a call (naming the
// call and its position), TS-F01 when it came from fn's own instructions.
func reportWidening(pass *analysis.Pass, pin pins.Pin, v violation) {
	found := firstWidenedOrigin(v.Fn, v.Val, v.Widened)
	fix := directive.Format(directive.Directive{
		Verb: "effects", Args: directive.FormatEffects(v.Rendered),
	})

	if found.Origin.Kind == sourceCall {
		pass.Report(analysis.Diagnostic{
			Pos:      pin.Pos,
			Category: "TS-F02",
			Message: fmt.Sprintf(
				"TS-F02: computed effects %s are not declared by this pin — introduced "+
					"by a call to %s at %s — remove the call or update the pin to %s",
				directive.FormatEffects(v.Widened), found.Origin.Detail,
				nearPos(pass.Fset, found.Origin.Pos), fix,
			),
		})
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      pin.Pos,
		Category: "TS-F01",
		Message: fmt.Sprintf(
			"TS-F01: computed effects %s are not declared by this pin — introduced by "+
				"%s — remove it or update the pin to %s",
			directive.FormatEffects(v.Widened), found.Origin.Detail, fix,
		),
	})
}

// located names one widened atom or mutate location and the source that
// first introduced it.
type located struct {
	Key    string
	Origin source
}

// firstWidenedOrigin picks the representative widened atom in canonical
// lattice order (bare atoms and IO qualifiers, then mutate locations) and
// returns its recorded origin.
func firstWidenedOrigin(fn *ssa.Function, val accum, widened directive.EffectSet) located {
	bare := widened
	bare.Mutate = nil
	for _, key := range atomsPresent(bare) {
		if origin, ok := val.Origins[key]; ok {
			return located{Key: key, Origin: origin}
		}
	}
	for _, name := range widened.Mutate {
		if origin, ok := originForMutateName(fn, val, name); ok {
			return located{Key: name, Origin: origin}
		}
	}
	assert.Unreachable("widened atom carries no recorded origin")
	return located{}
}

// originForMutateName finds the origin recorded for the location that
// renders as name through fn's own parameters.
func originForMutateName(fn *ssa.Function, val accum, name string) (source, bool) {
	for _, loc := range val.Mutate {
		rendered, ok := ssalib.RenderLocation(fn, loc)
		if !ok {
			continue
		}
		if rendered != name {
			continue
		}
		origin, ok := val.Origins[mutateKey(loc)]
		return origin, ok
	}
	return source{}, false
}

// nearPos prints a position with only the file's base name: the driver
// relativizes finding positions but never message text, so an absolute
// path here would reach the output and break cross-machine determinism.
func nearPos(fset *token.FileSet, pos token.Pos) string {
	at := fset.Position(pos)
	return fmt.Sprintf("%s:%d:%d", filepath.Base(at.Filename), at.Line, at.Column)
}
