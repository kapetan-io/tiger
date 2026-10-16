// Package ssalib is the shared SSA fragment under the analyzers that walk
// the call graph (norecursion): static-call enumeration and the iterative
// strongly-connected-components walk.
//
// Everything is deterministic, so results follow SSA block and instruction
// order, never map iteration. Everything under-approximates, so a callee that
// is not static contributes nothing and an analyzer built on this package
// resolves undecidability to a known-miss, never a finding.
package ssalib

import (
	"go/token"

	"golang.org/x/tools/go/ssa"
)

// StaticCall is one call whose callee is statically known: a direct
// function call, a method call on a concrete type, or an immediately
// invoked or spawned function literal.
type StaticCall struct {
	Callee  *ssa.Function
	Pos     token.Pos
	Args    []ssa.Value
	Spawned bool
}

// StaticCalls enumerates fn's statically resolvable calls in block and
// instruction order. Calls through interfaces or unresolved function
// values are absent — the known-miss boundary, never guessed at.
func StaticCalls(fn *ssa.Function) []StaticCall {
	calls := []StaticCall{}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			site, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			callee := site.Common().StaticCallee()
			if callee == nil {
				continue
			}
			_, spawned := instr.(*ssa.Go)
			calls = append(calls, StaticCall{
				Callee:  callee,
				Pos:     instr.Pos(),
				Args:    site.Common().Args,
				Spawned: spawned,
			})
		}
	}
	return calls
}
