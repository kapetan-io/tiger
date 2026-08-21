// Package ssalib is the shared SSA fragment under the effects-family
// analyzers (effects, frames, norecursion, poolzero), mirroring the words
// precedent: one place for the stdlib effects table, static-call
// enumeration, parameter-rooted write paths, and the iterative
// strongly-connected-components walk.
//
// Two contracts hold throughout. Everything is deterministic — results
// follow SSA block and instruction order, never map iteration — because
// fact content feeds the CI double-run diff. And everything
// under-approximates: an address the chase cannot follow or a callee that
// is not static contributes nothing, so an analyzer built on this package
// resolves undecidability to a known-miss, never a finding.
package ssalib

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Location is one parameter-rooted write target in index form: the
// parameter's position (the receiver is parameter 0 of a method) and the
// dotted field path beneath it, "" for the parameter's own pointee. Index
// form is what facts carry across packages; names are rebound per function
// at render time.
type Location struct {
	Param int
	Path  string
}

// Write is one parameter-rooted store, located for provenance.
type Write struct {
	Loc Location
	Pos token.Pos
}

// StaticCall is one call whose callee is statically known: a direct
// function call, a method call on a concrete type, or an immediately
// invoked or spawned function literal.
type StaticCall struct {
	Callee  *ssa.Function
	Pos     token.Pos
	Args    []ssa.Value
	Spawned bool
}

// Origin locates the instruction that first contributed an effect atom,
// so a pin violation can name what introduced the effect.
type Origin struct {
	Pos    token.Pos
	Detail string
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

// Writes returns fn's parameter-rooted writes in block and instruction
// order: stores and map updates whose target chases through field
// selections, element addresses, and pointer loads to a parameter or the
// receiver. Writes to locals, and targets the chase cannot follow
// (aliases, globals), are dropped.
func Writes(fn *ssa.Function) []Write {
	found := []Write{}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			target := writeTarget(instr)
			if target == nil {
				continue
			}
			loc, ok := chase(target)
			if !ok {
				continue
			}
			found = append(found, Write{Loc: loc, Pos: instr.Pos()})
		}
	}
	return found
}

// writeTarget returns the value an instruction writes through, or nil for
// instructions that write nothing.
func writeTarget(instr ssa.Instruction) ssa.Value {
	switch write := instr.(type) {
	case *ssa.Store:
		return write.Addr
	case *ssa.MapUpdate:
		return write.Map
	default:
		return nil
	}
}

// chase follows a write target up through the address chain to a
// parameter root. The chain is finite in well-formed SSA; the fixed bound
// converts a malformed chain into a silent drop instead of a hang.
func chase(target ssa.Value) (Location, bool) {
	path := ""
	for depth := 0; depth < 1000; depth++ {
		switch at := target.(type) {
		case *ssa.Parameter:
			position, ok := paramIndex(at)
			if !ok {
				return Location{}, false
			}
			return Location{Param: position, Path: path}, true
		case *ssa.FieldAddr:
			name, ok := fieldName(at)
			if !ok {
				return Location{}, false
			}
			path = "." + name + path
			target = at.X
		case *ssa.IndexAddr:
			// Writing an element is writing the container; the path
			// names the container.
			target = at.X
		case *ssa.UnOp:
			if at.Op != token.MUL {
				return Location{}, false
			}
			target = at.X
		default:
			return Location{}, false
		}
	}
	return Location{}, false
}

// fieldName resolves a field address to its field's name.
func fieldName(addr *ssa.FieldAddr) (string, bool) {
	pointer, ok := addr.X.Type().Underlying().(*types.Pointer)
	if !ok {
		return "", false
	}
	shape, ok := pointer.Elem().Underlying().(*types.Struct)
	if !ok {
		return "", false
	}
	return shape.Field(addr.Field).Name(), true
}

// paramIndex locates a parameter's position in its function's signature;
// the receiver of a method is position 0.
func paramIndex(param *ssa.Parameter) (int, bool) {
	for i, candidate := range param.Parent().Params {
		if candidate == param {
			return i, true
		}
	}
	return 0, false
}

// RenderLocation prints a location through fn's own parameter names —
// "r.log" — the form pin syntax uses. An out-of-range or unnamed
// parameter cannot appear in a pin, so it does not render.
func RenderLocation(fn *ssa.Function, loc Location) (string, bool) {
	if loc.Param >= len(fn.Params) {
		return "", false
	}
	name := fn.Params[loc.Param].Name()
	if name == "" {
		return "", false
	}
	if name == "_" {
		return "", false
	}
	return name + loc.Path, true
}

// RebaseArg maps a callee location through a call's arguments into the
// caller's frame: when the caller passes its own parameter straight
// through, the callee's write is the caller's write. Any other argument
// shape is a caller-local mutation or an alias — dropped, a known-miss.
func RebaseArg(args []ssa.Value, loc Location) (Location, bool) {
	if loc.Param >= len(args) {
		return Location{}, false
	}
	param, ok := args[loc.Param].(*ssa.Parameter)
	if !ok {
		return Location{}, false
	}
	position, ok := paramIndex(param)
	if !ok {
		return Location{}, false
	}
	return Location{Param: position, Path: loc.Path}, true
}
