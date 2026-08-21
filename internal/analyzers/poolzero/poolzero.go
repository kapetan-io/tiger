// Package poolzero enforces TS-M05: a type reused through a sync.Pool must
// implement Reset, and every Put is preceded by a reset of the exact value
// put, on every control-flow path reaching it.
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
// a reset counts only when its receiver or store address is the SAME
// ssa.Value as the argument boxed into Put, chased through
// *ssa.MakeInterface with no alias analysis. Three shapes fall outside what
// this can prove, and all three stay silent rather than risk a false
// positive on a blocking rule: a pooled value reset in one function and put
// in another (the reset is invisible from the Put's function alone, so a
// function containing no reset instruction for the Put's exact value is
// treated as this shape and never fires — unless the value provably came
// from this pool's own Get, which no caller could have cleaned, so a
// reset-free Put of it is the rule's primary failure mode and fires); the
// same split read backward —
// reset in the callee, put in the caller — is a real bleed this analyzer
// also cannot see; and an aliased pooled value, reset through one
// SSA-level name and put through a different one that happens to hold the
// same object at runtime.
package poolzero

import (
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/assert"
)

const (
	resetName = "Reset"

	msgNoResetFmt = "TS-M05: %s is put into a sync.Pool but has no Reset method — implement " +
		"Reset() so zeroing on release is a single, checkable call"
	msgNotReset = "TS-M05: this Put is not preceded by a reset of the same value on every path " +
		"— call Reset() (or zero the value) before every Put so a leak cannot outlive the owner " +
		"who knew what was sensitive"
)

// Analyzer enforces TS-M05: pooled types implement Reset, and every Put is
// preceded by a reset of the same value on every path.
var Analyzer = &analysis.Analyzer{
	Name:     "poolzero",
	Doc:      "TS-M05: pooled types implement Reset, and every sync.Pool Put is reset-dominated.",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

// putSite is one static call to (*sync.Pool).Put, located within its block
// so the reset-dominance check can order it against same-block resets.
type putSite struct {
	Instr ssa.CallInstruction
	Block *ssa.BasicBlock
	Arg   ssa.Value
}

// boxed is the concrete operand and type a *ssa.MakeInterface carries.
type boxed struct {
	X    ssa.Value
	Type types.Type
}

// reachState carries the must-reach fixpoint's per-block facts for one
// (value, Put) pair.
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

// checkFunc runs both TS-M05 checks over every sync.Pool Put call site in
// fn. The buildssa.SrcFuncs list already includes every function literal
// nested inside fn, at every depth, so no separate AnonFuncs walk is
// needed.
func checkFunc(pass *analysis.Pass, fn *ssa.Function) {
	for _, put := range putSites(fn) {
		box, ok := chaseBox(put.Arg)
		if !ok {
			continue
		}
		if !hasResetMethod(pass, box.Type) {
			pass.Report(analysis.Diagnostic{
				Pos:      put.Instr.Pos(),
				Category: "TS-M05",
				Message:  fmt.Sprintf(msgNoResetFmt, box.Type.String()),
			})
			continue
		}
		if !anyReset(fn, box.X) {
			// A value taken straight from the pool is never known-clean, so
			// the interprocedural silence below does not shelter it.
			if !fromPoolGet(box.X) {
				continue
			}
		}
		if mustReach(fn, box.X, put) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      put.Instr.Pos(),
			Category: "TS-M05",
			Message:  msgNotReset,
		})
	}
}

// putSites enumerates fn's static (*sync.Pool).Put call sites in block and
// instruction order. This walks fn.Blocks directly, rather than through the
// shared ssalib.StaticCalls summary, because the dominance check below
// needs the instruction and block identity a summary type does not retain.
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
			sites = append(sites, putSite{Instr: site, Block: block, Arg: call.Args[1]})
		}
	}
	return sites
}

// chaseBox unwraps the *ssa.MakeInterface that boxes a value for an any
// parameter, returning the concrete operand and its type. A put argument
// that is not directly a MakeInterface — a value the chase cannot follow —
// reports false, the known-miss path.
func chaseBox(v ssa.Value) (boxed, bool) {
	box, ok := v.(*ssa.MakeInterface)
	if !ok {
		return boxed{}, false
	}
	return boxed{X: box.X, Type: box.X.Type()}, true
}

// hasResetMethod reports whether t, dereferenced if it is a pointer, has a
// method named Reset on either its value or pointer method set.
func hasResetMethod(pass *analysis.Pass, t types.Type) bool {
	lookup := t
	if ptr, ok := lookup.(*types.Pointer); ok {
		lookup = ptr.Elem()
	}
	obj, _, _ := types.LookupFieldOrMethod(lookup, true, pass.Pkg, resetName)
	_, ok := obj.(*types.Func)
	return ok
}

// anyReset reports whether fn resets x anywhere, in any block. A function
// with no reset of x at all is the interprocedural known-miss: the reset
// may already have happened in the caller, or through an alias this
// analyzer's identity-exact check cannot follow — either way, firing would
// risk a false positive on a blocking rule, so this stays silent rather
// than run the dominance check below.
func anyReset(fn *ssa.Function, x ssa.Value) bool {
	for _, block := range fn.Blocks {
		if blockGen(block, x, nil) {
			return true
		}
	}
	return false
}

// mustReach reports whether a reset of x reaches put on every control-flow
// path from fn's entry block, via a forward dataflow to a bounded fixpoint:
// IN[entry] is false, IN[b] is the AND of every predecessor's OUT, and
// OUT[b] is IN[b] OR whether b resets x. Each block's IN/OUT can only flip
// false to true, never back, so len(fn.Blocks)+2 rounds always reaches a
// stable point.
func mustReach(fn *ssa.Function, x ssa.Value, put putSite) bool {
	state := &reachState{
		entry:  fn.Blocks[0],
		x:      x,
		put:    put,
		input:  map[*ssa.BasicBlock]bool{},
		output: map[*ssa.BasicBlock]bool{},
	}
	for round := 0; round < len(fn.Blocks)+2; round++ {
		if !state.step(fn.Blocks) {
			break
		}
	}
	return state.input[put.Block] || state.gen(put.Block)
}

// step runs one fixpoint round over blocks, reporting whether any input or
// output value changed.
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

// computeInput is false for the entry block and for a block with no
// predecessors, otherwise the AND of every predecessor's current output.
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

// gen reports whether block resets x, stopping the scan at the Put
// instruction when block is the Put's own block — a reset placed after the
// Put in program order does not count.
func (r *reachState) gen(block *ssa.BasicBlock) bool {
	var stop ssa.Instruction
	if block == r.put.Block {
		stop = r.put.Instr
	}
	return blockGen(block, r.x, stop)
}

// blockGen scans block's instructions in order for a reset of x, stopping
// before stop when stop is non-nil and present in this block.
func blockGen(block *ssa.BasicBlock, x ssa.Value, stop ssa.Instruction) bool {
	for _, instr := range block.Instrs {
		if instr == stop {
			return false
		}
		if isReset(instr, x) {
			return true
		}
	}
	return false
}

// isReset reports whether instr resets x: a static call to x's Reset
// method with x as the receiver, or a store through x of a zero value.
func isReset(instr ssa.Instruction, x ssa.Value) bool {
	if isResetCall(instr, x) {
		return true
	}
	store, ok := instr.(*ssa.Store)
	if !ok {
		return false
	}
	if store.Addr != x {
		return false
	}
	return isZeroed(store.Val)
}

// isResetCall reports whether instr is a static method call named Reset
// whose receiver is x.
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

// isZeroed recognizes the two canonical zeroing-store forms: a nil or zero
// constant, or a load of a freshly zero-Alloc'd local composite (the SSA
// shape of a `T{}` literal assigned through a pointer). Anything cleverer
// is not counted as a reset — a deliberate strictness this package's
// corpus documents, matching maporder's closed allowlist, since the
// compliant form (call Reset before Put) is always available.
func isZeroed(v ssa.Value) bool {
	switch operand := v.(type) {
	case *ssa.Const:
		return operand.IsNil()
	case *ssa.UnOp:
		if operand.Op != token.MUL {
			return false
		}
		_, isAlloc := operand.X.(*ssa.Alloc)
		return isAlloc
	default:
		return false
	}
}

// fromPoolGet reports whether x originates from a (*sync.Pool).Get call,
// chased through the type assertion and tuple extraction Get's any result
// forces. A Get-obtained value carries a previous user's contents, so a
// Put of one with no reset anywhere is the rule's primary failure mode —
// unlike a parameter, no caller could have cleaned it first.
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
