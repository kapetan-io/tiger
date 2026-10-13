// Package sametypeparams enforces TS-N07 (adjacent parameters of identical
// type, and more than four parameters not counting a leading t or ctx, need an options struct) and TS-N08
// (a plain bool parameter needs a named type).
//
// The spec's own assert package — Ok(cond bool, ...), Equal(got, want T) —
// is tiger infrastructure whose API the spec itself fixes, so the assert
// package and its external test package are exempt from both rules. Every
// other declared function signature in the package is covered: func
// declarations and interface methods. Function literals are exempt — their
// signatures are dictated by the API that consumes them (sort.Slice's
// func(i, j int) bool cannot be restructured into an options struct), so
// flagging them was a false positive on a blocking rule, surfaced by this
// repository's own dogfood run and fixed here per correctness constraint 7.
//
// The four-parameter limit does not count a leading testing.TB, *testing.T
// or *testing.B, nor a context.Context first or right after it (ADR-0018).
// The adjacent-same-type half counts every parameter.
package sametypeparams

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-N07 and TS-N08: no adjacent same-type parameters, no bool parameters.
var Analyzer = &analysis.Analyzer{
	Name: "sametypeparams",
	Doc:  "TS-N07 and TS-N08: no adjacent same-type parameters, no bool parameters.",
	Run:  run,
}

const maxParams = 4

// paramSlot is one flattened parameter: a field with N names contributes N
// slots sharing that field's type, and an unnamed field contributes one.
type paramSlot struct {
	typ types.Type
	pos token.Pos
}

func run(pass *analysis.Pass) (any, error) {
	pkg := pass.Pkg.Name()
	if pkg == "assert" || pkg == "assert_test" {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch decl := node.(type) {
			case *ast.FuncDecl:
				checkParams(pass, decl.Type)
			case *ast.InterfaceType:
				for _, method := range decl.Methods.List {
					if funcType, ok := method.Type.(*ast.FuncType); ok {
						checkParams(pass, funcType)
					}
				}
			}
			return true
		})
	}
	return nil, nil
}

// checkParams reports TS-N07 for an over-long parameter list and for each
// adjacent same-type pair, and TS-N08 for each plain bool parameter.
func checkParams(pass *analysis.Pass, funcType *ast.FuncType) {
	slots := flattenParams(pass, funcType.Params)
	exempt := leadingExempt(slots)
	if counted := len(slots) - exempt; counted > maxParams {
		suffix := ""
		if exempt > 0 {
			suffix = " besides its leading t or ctx"
		}
		pass.Report(analysis.Diagnostic{
			Pos:      funcType.Params.Pos(),
			Category: "TS-N07",
			Message: fmt.Sprintf("TS-N07: this function takes %d parameters%s — group the "+
				"related ones into an options struct", counted, suffix),
		})
	}
	for i := 0; i < len(slots)-1; i++ {
		checkAdjacent(pass, slots, i)
	}
	for _, slot := range slots {
		checkBoolParam(pass, slot)
	}
}

// leadingExempt returns how many leading slots the parameter limit ignores.
func leadingExempt(slots []paramSlot) int {
	exempt := 0
	if len(slots) > 0 && isTestingHandle(slots[0].typ) {
		exempt = 1
	}
	if len(slots) > exempt && isNamed(slots[exempt].typ, "context.Context") {
		exempt++
	}
	return exempt
}

// isTestingHandle reports whether typ is testing.TB, *testing.T or *testing.B.
func isTestingHandle(typ types.Type) bool {
	if ptr, ok := types.Unalias(typ).(*types.Pointer); ok {
		return isNamed(ptr.Elem(), "testing.T") || isNamed(ptr.Elem(), "testing.B")
	}
	return isNamed(typ, "testing.TB")
}

// isNamed reports whether typ is the named type with the import-path
// qualified name, such as "context.Context".
func isNamed(typ types.Type, qualified string) bool {
	named, ok := types.Unalias(typ).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path()+"."+obj.Name() == qualified
}

// checkAdjacent reports TS-N07 when the parameter at index and the one
// after it share an identical declared type.
func checkAdjacent(pass *analysis.Pass, slots []paramSlot, index int) {
	current, next := slots[index], slots[index+1]
	if current.typ == nil || next.typ == nil || !types.Identical(current.typ, next.typ) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      next.pos,
		Category: "TS-N07",
		Message: fmt.Sprintf("TS-N07: adjacent parameters share type %s, so a caller can swap "+
			"them and nothing complains — use an options struct, or give each a distinct "+
			"named type", current.typ.String()),
	})
}

// checkBoolParam reports TS-N08 when slot's type is exactly the
// predeclared bool.
func checkBoolParam(pass *analysis.Pass, slot paramSlot) {
	basic, ok := slot.typ.(*types.Basic)
	if !ok || basic.Kind() != types.Bool {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      slot.pos,
		Category: "TS-N08",
		Message: "TS-N08: this parameter is a plain bool, so a call like Save(true) says " +
			"nothing at the call site — define a named bool type (type SyncMode bool) with " +
			"named constants, so the call reads Save(SyncImmediate)",
	})
}

// flattenParams expands a parameter field list into one slot per parameter
// value, preserving declaration order.
func flattenParams(pass *analysis.Pass, list *ast.FieldList) []paramSlot {
	if list == nil {
		return nil
	}
	slots := make([]paramSlot, 0, list.NumFields())
	for _, field := range list.List {
		typ := pass.TypesInfo.TypeOf(field.Type)
		if len(field.Names) == 0 {
			slots = append(slots, paramSlot{typ: typ, pos: field.Pos()})
			continue
		}
		for _, name := range field.Names {
			slots = append(slots, paramSlot{typ: typ, pos: name.Pos()})
		}
	}
	return slots
}
