// Package poolzero enforces TS-M05: every Put into a sync.Pool is preceded by
// zeroing the exact value put, with *b = T{}, on every control-flow path
// reaching it. A Reset method does not count, because nothing makes it clear
// anything: an empty Reset passes any check that trusts the name.
//
// Why. Go zeroes every make, so the classic buffer-bleed case is already
// gone — but a sync.Pool exists specifically to skip that zeroing on reuse.
// Reuse resurrects the bleed Go's allocator otherwise prevents: a struct or
// byte slice that held one caller's sensitive data comes back, unzeroed,
// for the next caller who never touched it. Zeroing on acquire only
// protects the *next* holder; the data already sat resident, reachable
// through whoever kept a reference after Put returned. Zeroing on release
// is the only point that bounds the leak to the owner who knew what was
// sensitive.
//
// SSA dominance turns "preceded by" from a code-review judgment call into
// an exact, per-path fact. A forward must-reach dataflow over the
// function's blocks — one round per block, to a bounded fixpoint — proves a
// reset happens on every control-flow path from entry to the Put, not just
// the one path a reviewer happened to trace; an if with a reset in only one
// branch is caught even though a single-dominator check would miss it.
//
// Known-miss boundary. The analysis is intraprocedural and identity-exact:
// a zeroing counts only when its store address is the SAME ssa.Value as the
// argument boxed into Put, chased through *ssa.MakeInterface with no alias
// analysis. Three shapes fall outside what this can prove, and all three stay
// silent rather than risk a false positive on a blocking rule: a pooled value
// zeroed in one function and put in another (the zeroing is invisible from the
// Put's function alone, so a function containing no zeroing or Reset call for
// the Put's exact value is treated as this shape and never fires — unless the
// value provably came from this pool's own Get, which no caller could have
// cleaned, so a Put of it with no zeroing is the rule's primary failure mode
// and fires); the same split read backward — zeroing in the callee, put in
// the caller — is a real bleed this analyzer also cannot see; and an aliased
// pooled value, zeroed through one SSA-level name and put through a different
// one that happens to hold the same object at runtime.
package poolzero

import (
	"fmt"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/worklist"
)

const (
	resetName = "Reset"

	msgByValueFmt = "TS-M05: %s is put into a sync.Pool by value, so it can't be zeroed in " +
		"place — put a pointer and write *b = T{} before every Put"
	msgNotReset = "TS-M05: this Put is not preceded by *b = T{} on every path to it — zero " +
		"the value before every Put, because a Reset method can be empty and pooled data " +
		"would leak to the next user"
)

var Analyzer = &analysis.Analyzer{
	Name:     "poolzero",
	Doc:      "TS-M05: every sync.Pool Put is preceded by *b = T{} on every path.",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

type putSite struct {
	Instr ssa.CallInstruction
	At    ssa.Instruction
	Block *ssa.BasicBlock
	Arg   ssa.Value
}

type boxed struct {
	X    ssa.Value
	Type types.Type
}

type reachState struct {
	entry  *ssa.BasicBlock
	x      ssa.Value
	put    putSite
	input  map[*ssa.BasicBlock]bool
	output map[*ssa.BasicBlock]bool
}

func run(pass *analysis.Pass) (any, error) {
	built, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	assert.Ok(ok, "buildssa result missing")
	for _, fn := range built.SrcFuncs {
		checkFunc(pass, fn)
	}
	return nil, nil
}

func checkFunc(pass *analysis.Pass, fn *ssa.Function) {
	reported := map[token.Pos]bool{}
	for _, put := range putSites(fn) {
		if reported[put.Instr.Pos()] {
			continue
		}
		box, ok := chaseBox(put.Arg)
		if !ok {
			continue
		}
		if _, isPointer := box.Type.Underlying().(*types.Pointer); !isPointer {
			pass.Report(analysis.Diagnostic{
				Pos:      put.Instr.Pos(),
				Category: "TS-M05",
				Message:  fmt.Sprintf(msgByValueFmt, box.Type.String()),
			})
			continue
		}
		if !handlesValue(fn, box.X) {
			if !fromPoolGet(box.X) {
				continue
			}
		}
		if mustReach(fn, box.X, put) {
			continue
		}
		reported[put.Instr.Pos()] = true
		pass.Report(analysis.Diagnostic{
			Pos:      put.Instr.Pos(),
			Category: "TS-M05",
			Message:  msgNotReset,
		})
	}
}

func putSites(fn *ssa.Function) []putSite {
	sites := []putSite{}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			site, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			call := site.Common()
			callee := call.StaticCallee()
			if callee == nil {
				continue
			}
			if callee.String() != "(*sync.Pool).Put" {
				continue
			}
			if len(call.Args) < 2 {
				continue
			}
			if _, deferred := instr.(*ssa.Defer); deferred {
				sites = append(sites, deferredSites(fn, site, call.Args[1])...)
				continue
			}
			sites = append(sites, putSite{Instr: site, At: instr, Block: block, Arg: call.Args[1]})
		}
	}
	return sites
}

func deferredSites(fn *ssa.Function, deferred ssa.CallInstruction, arg ssa.Value) []putSite {
	after := reachableAfter(deferred)
	sites := []putSite{}
	for _, block := range fn.Blocks {
		passed := false
		for _, instr := range block.Instrs {
			if instr == ssa.Instruction(deferred) {
				passed = true
			}
			if _, runs := instr.(*ssa.RunDefers); !runs {
				continue
			}
			if block == deferred.Block() && !passed {
				continue
			}
			if block != deferred.Block() && !after[block] {
				continue
			}
			sites = append(sites, putSite{Instr: deferred, At: instr, Block: block, Arg: arg})
		}
	}
	return sites
}

func reachableAfter(deferred ssa.CallInstruction) map[*ssa.BasicBlock]bool {
	seen := map[*ssa.BasicBlock]bool{}
	work := slices.Clone(deferred.Block().Succs)
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		if !seen[work[i]] {
			seen[work[i]] = true
			work = append(work, work[i].Succs...)
		}
	}
	assert.Ok(i == len(work), "reachableAfter: worklist reached worklist.NodesMax")
	return seen
}

func chaseBox(v ssa.Value) (boxed, bool) {
	box, ok := v.(*ssa.MakeInterface)
	if !ok {
		return boxed{}, false
	}
	return boxed{X: box.X, Type: box.X.Type()}, true
}

func handlesValue(fn *ssa.Function, x ssa.Value) bool {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if isZeroing(instr, x) || isResetCall(instr, x) {
				return true
			}
		}
	}
	return false
}

func mustReach(fn *ssa.Function, x ssa.Value, put putSite) bool {
	state := &reachState{
		entry:  fn.Blocks[0],
		x:      x,
		put:    put,
		input:  map[*ssa.BasicBlock]bool{},
		output: map[*ssa.BasicBlock]bool{},
	}
	settled := false
	for round := 0; round < len(fn.Blocks)+2; round++ {
		if !state.step(fn.Blocks) {
			settled = true
			break
		}
	}
	assert.Ok(settled, "mustReach: no fixpoint within len(fn.Blocks)+2 rounds")
	return state.input[put.Block] || state.gen(put.Block)
}

func (r *reachState) step(blocks []*ssa.BasicBlock) bool {
	changed := false
	for _, block := range blocks {
		nextIn := r.computeInput(block)
		if nextIn != r.input[block] {
			r.input[block] = nextIn
			changed = true
		}
		nextOut := r.input[block] || r.gen(block)
		if nextOut != r.output[block] {
			r.output[block] = nextOut
			changed = true
		}
	}
	return changed
}

func (r *reachState) computeInput(block *ssa.BasicBlock) bool {
	if block == r.entry {
		return false
	}
	if len(block.Preds) == 0 {
		return false
	}
	for _, pred := range block.Preds {
		if !r.output[pred] {
			return false
		}
	}
	return true
}

func (r *reachState) gen(block *ssa.BasicBlock) bool {
	var stop ssa.Instruction
	if block == r.put.Block {
		stop = r.put.At
	}
	return blockGen(block, r.x, stop)
}

func blockGen(block *ssa.BasicBlock, x ssa.Value, stop ssa.Instruction) bool {
	for _, instr := range block.Instrs {
		if instr == stop {
			return false
		}
		if isZeroing(instr, x) {
			return true
		}
	}
	return false
}

func isZeroing(instr ssa.Instruction, x ssa.Value) bool {
	store, ok := instr.(*ssa.Store)
	if !ok {
		return false
	}
	if store.Addr != x {
		return false
	}
	return isZeroed(store.Val)
}

func isResetCall(instr ssa.Instruction, x ssa.Value) bool {
	site, ok := instr.(ssa.CallInstruction)
	if !ok {
		return false
	}
	call := site.Common()
	callee := call.StaticCallee()
	if callee == nil {
		return false
	}
	if callee.Name() != resetName {
		return false
	}
	if callee.Signature.Recv() == nil {
		return false
	}
	if len(call.Args) == 0 {
		return false
	}
	return call.Args[0] == x
}

func isZeroed(v ssa.Value) bool {
	switch operand := v.(type) {
	case *ssa.Const:
		return operand.Value == nil
	case *ssa.UnOp:
		if operand.Op != token.MUL {
			return false
		}
		alloc, isAlloc := operand.X.(*ssa.Alloc)
		return isAlloc && untouched(alloc, operand)
	default:
		return false
	}
}

func untouched(alloc *ssa.Alloc, load *ssa.UnOp) bool {
	if alloc.Comment != "complit" {
		return false
	}
	for _, ref := range *alloc.Referrers() {
		if ref == ssa.Instruction(load) || zeroFieldAddr(ref) {
			continue
		}
		if _, debug := ref.(*ssa.DebugRef); !debug {
			return false
		}
	}
	return true
}

func zeroFieldAddr(ref ssa.Instruction) bool {
	addr, ok := ref.(*ssa.FieldAddr)
	if !ok {
		return false
	}
	for _, use := range *addr.Referrers() {
		store, isStore := use.(*ssa.Store)
		if !isStore {
			return false
		}
		if zero, isConst := store.Val.(*ssa.Const); !isConst || zero.Value != nil {
			return false
		}
	}
	return true
}

func fromPoolGet(x ssa.Value) bool {
	for range 64 {
		switch at := x.(type) {
		case *ssa.Extract:
			x = at.Tuple
		case *ssa.TypeAssert:
			x = at.X
		case *ssa.ChangeInterface:
			x = at.X
		case *ssa.MakeInterface:
			x = at.X
		case *ssa.Call:
			callee := at.Common().StaticCallee()
			if callee == nil {
				return false
			}
			return callee.String() == "(*sync.Pool).Get"
		default:
			return false
		}
	}
	return false
}
