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

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/assert"
)

const (
	resetName = "Reset"

	msgByValueFmt = "TS-M05: %s is put into a sync.Pool by value, so it can't be zeroed in " +
		"place — put a pointer and write *b = T{} before every Put"
	msgNotReset = "TS-M05: this Put is not preceded by *b = T{} on every path to it — zero " +
		"the value before every Put, because a Reset method can be empty and pooled data " +
		"would leak to the next user"
)

// Analyzer enforces TS-M05: every Put is preceded by *b = T{} on the same
// value on every path.
var Analyzer = &analysis.Analyzer{
	Name:     "poolzero",
	Doc:      "TS-M05: every sync.Pool Put is preceded by *b = T{} on every path.",
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
		if _, isPointer := box.Type.Underlying().(*types.Pointer); !isPointer {
			pass.Report(analysis.Diagnostic{
				Pos:      put.Instr.Pos(),
				Category: "TS-M05",
				Message:  fmt.Sprintf(msgByValueFmt, box.Type.String()),
			})
			continue
		}
		if !handlesValue(fn, box.X) {
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

// handlesValue reports whether fn zeroes x, or calls a Reset method on it, in
// any block. A Reset call does not count as a zeroing, but it shows the
// function means to clean x itself, so the dominance check below judges it.
// A function that does neither is the interprocedural known-miss: the
// zeroing may already have happened in the caller, or through an alias this
// analyzer's identity-exact check cannot follow — either way, firing would
// risk a false positive on a blocking rule, so this stays silent rather
// than run the dominance check below.
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
		if isZeroing(instr, x) {
			return true
		}
	}
	return false
}

// isZeroing reports whether instr is a store through x of a zero value, the
// SSA form of *b = T{}.
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

// isZeroed recognizes the two canonical zeroing-store forms: the zero
// constant of any type (the SSA shape of an empty `T{}` assigned through a
// pointer, a Const with no value), or a load of a freshly zero-Alloc'd local
// composite. Anything cleverer is not counted as a zeroing — a deliberate
// strictness this package's corpus documents, since the compliant form
// (`*b = T{}` before Put) is always available.
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

// untouched reports whether the composite literal alloc backs is read only by
// load and written nowhere, so it still holds the zero value. A literal with a
// field set, or a local filled in before `*b = t`, is not a zeroing.
func untouched(alloc *ssa.Alloc, load *ssa.UnOp) bool {
	if alloc.Comment != "complit" {
		return false
	}
	for _, ref := range *alloc.Referrers() {
		if ref != ssa.Instruction(load) {
			if _, debug := ref.(*ssa.DebugRef); !debug {
				return false
			}
		}
	}
	return true
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
