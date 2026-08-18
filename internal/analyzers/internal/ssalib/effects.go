package ssalib

import (
	"go/token"

	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/internal/directive"
)

// Contribution is what a function's own instructions add to its effect
// set: the atoms present and, per atom, the first contributing
// instruction (in block and instruction order) for provenance.
type Contribution struct {
	Set     directive.EffectSet
	Origins map[string]Origin
}

// InstructionEffects computes the effects fn's own instructions contribute
// — spawn, alloc, block, panic. Calls, mutate paths, and the stdlib table
// compose on top in the effects analyzer.
func InstructionEffects(fn *ssa.Function) Contribution {
	found := Contribution{Origins: map[string]Origin{}}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			hit := instructionAtom(instr)
			if hit.atom == "" {
				continue
			}
			markBareAtom(&found.Set, hit.atom)
			if _, seen := found.Origins[hit.atom]; !seen {
				found.Origins[hit.atom] = Origin{Pos: instr.Pos(), Detail: hit.detail}
			}
		}
	}
	return found
}

// atomHit is one instruction's classified effect contribution; the zero
// value is no contribution.
type atomHit struct {
	atom   string
	detail string
}

// instructionAtom classifies one instruction's own effect contribution.
func instructionAtom(instr ssa.Instruction) atomHit {
	switch at := instr.(type) {
	case *ssa.Go:
		return atomHit{atom: "spawn", detail: "go statement"}
	case *ssa.Alloc:
		if at.Heap {
			return atomHit{atom: "alloc", detail: "heap allocation"}
		}
		return atomHit{}
	case *ssa.MakeSlice:
		return atomHit{atom: "alloc", detail: "make slice"}
	case *ssa.MakeMap:
		return atomHit{atom: "alloc", detail: "make map"}
	case *ssa.MakeChan:
		return atomHit{atom: "alloc", detail: "make chan"}
	case *ssa.MakeClosure:
		return atomHit{atom: "alloc", detail: "closure"}
	case *ssa.Panic:
		return atomHit{atom: "panic", detail: "panic"}
	case *ssa.Send:
		return atomHit{atom: "block", detail: "channel send"}
	case *ssa.UnOp:
		if receives(at) {
			return atomHit{atom: "block", detail: "channel receive"}
		}
		return atomHit{}
	case *ssa.Select:
		if at.Blocking {
			return atomHit{atom: "block", detail: "blocking select"}
		}
		return atomHit{}
	case *ssa.Call:
		if appends(at) {
			return atomHit{atom: "alloc", detail: "append"}
		}
		return atomHit{}
	default:
		return atomHit{}
	}
}

// receives reports whether a unary operation is a channel receive.
func receives(op *ssa.UnOp) bool {
	return op.Op == token.ARROW
}

// appends reports whether a call is the append builtin.
func appends(call *ssa.Call) bool {
	builtin, ok := call.Common().Value.(*ssa.Builtin)
	if !ok {
		return false
	}
	return builtin.Name() == "append"
}

// markBareAtom sets one unparameterized lattice member.
func markBareAtom(set *directive.EffectSet, atom string) {
	switch atom {
	case "alloc":
		set.Alloc = true
	case "block":
		set.Block = true
	case "panic":
		set.Panic = true
	case "spawn":
		set.Spawn = true
	}
}
