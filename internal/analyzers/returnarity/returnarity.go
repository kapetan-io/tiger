// Package returnarity enforces TS-E06: at most two results, and when there
// are exactly two, the second is error or bool.
//
// The spec's preference order is nothing, T, (T, bool), (T, error) — never
// (T, bool, error). The check covers every function signature in the
// package: func declarations, func literals, and interface methods.
package returnarity

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-E06: at most two results, and the second is an error or a bool.
var Analyzer = &analysis.Analyzer{
	Name: "returnarity",
	Doc:  "TS-E06: at most two results, and the second is an error or a bool.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch decl := node.(type) {
			case *ast.FuncDecl:
				checkResults(pass, decl.Type)
			case *ast.FuncLit:
				checkResults(pass, decl.Type)
			case *ast.InterfaceType:
				for _, method := range decl.Methods.List {
					if funcType, ok := method.Type.(*ast.FuncType); ok {
						checkResults(pass, funcType)
					}
				}
			}
			return true
		})
	}
	return nil, nil
}

// checkResults reports a TS-E06 finding for a signature with more than two
// results, or with exactly two whose last is neither error nor bool.
func checkResults(pass *analysis.Pass, funcType *ast.FuncType) {
	count := resultCount(funcType.Results)
	if count > 2 {
		pass.Report(analysis.Diagnostic{
			Pos:      funcType.Results.Pos(),
			Category: "TS-E06",
			Message: fmt.Sprintf("TS-E06: %d results — prefer nothing, T, (T, bool), or "+
				"(T, error); never (T, bool, error)", count),
		})
		return
	}
	if count != 2 {
		return
	}
	last := funcType.Results.List[len(funcType.Results.List)-1]
	lastType := pass.TypesInfo.TypeOf(last.Type)
	if lastType == nil || allowedSecond(lastType) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      last.Pos(),
		Category: "TS-E06",
		Message: fmt.Sprintf("TS-E06: second result is %s, not error or bool — prefer "+
			"(T, error) or (T, bool)", lastType.String()),
	})
}

// allowedSecond reports whether typ is exactly the predeclared bool or
// error type.
func allowedSecond(typ types.Type) bool {
	if basic, ok := typ.(*types.Basic); ok && basic.Kind() == types.Bool {
		return true
	}
	return types.Identical(typ, types.Universe.Lookup("error").Type())
}

// resultCount counts a signature's result values, treating a field with N
// names as N results and an unnamed field as one.
func resultCount(list *ast.FieldList) int {
	if list == nil {
		return 0
	}
	count := 0
	for _, field := range list.List {
		if len(field.Names) == 0 {
			count++
			continue
		}
		count += len(field.Names)
	}
	return count
}
