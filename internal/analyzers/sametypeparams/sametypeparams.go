// Package sametypeparams enforces TS-N07 (adjacent parameters of identical
// type, and more than four parameters, need an options struct) and TS-N08
// (a plain bool parameter needs a named type).
//
// The spec's own assert package — Ok(cond bool, ...), Equal(got, want T) —
// is dialect infrastructure whose API the spec itself fixes, so the assert
// package and its external test package are exempt from both rules. Every
// other declared function signature in the package is covered: func
// declarations and interface methods. Function literals are exempt — their
// signatures are dictated by the API that consumes them (sort.Slice's
// func(i, j int) bool cannot be restructured into an options struct), so
// flagging them was a false positive on a blocking rule, surfaced by this
// repository's own dogfood run and fixed here per correctness constraint 7.
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
	if len(slots) > maxParams {
		pass.Report(analysis.Diagnostic{
			Pos:      funcType.Params.Pos(),
			Category: "TS-N07",
			Message: fmt.Sprintf("TS-N07: %d parameters — group related parameters into an "+
				"options struct instead of growing the parameter list", len(slots)),
		})
	}
	for i := 0; i < len(slots)-1; i++ {
		checkAdjacent(pass, slots, i)
	}
	for _, slot := range slots {
		checkBoolParam(pass, slot)
	}
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
		Message: fmt.Sprintf("TS-N07: adjacent parameters share type %s — Go has no named "+
			"arguments, so a swapped call is silent; use an options struct or distinct named "+
			"types", current.typ.String()),
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
		Message: "TS-N08: plain bool parameter — Save(true) means nothing at the call site; " +
			"define a named type with a bool underlying type (for example type SyncMode bool) " +
			"so Save(SyncImmediate) means something",
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
