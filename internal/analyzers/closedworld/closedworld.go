// Package closedworld enforces TS-K03: in a package declaring
// //tiger:restrict closed-dispatch, every interface method call devirtualizes
// to exactly one concrete type.
//
// The check is intra-function and intra-package only, by rule, not by
// omission: a receiver built in a helper and returned, or one that could
// resolve to a concrete type through cross-package analysis, is still a
// finding, because the package claimed closure and the call site is where
// a reader needs to see that the claim does not hold. Devirtualization
// through function returns, struct fields, or across packages is future
// work if trial evidence shows that shape dominating (see the blueprint's
// Limitations section) — it is not a gap this analyzer papers over.
package closedworld

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/restrict"
)

// Analyzer enforces TS-K03.
var Analyzer = &analysis.Analyzer{
	Name: "closedworld",
	Doc: "TS-K03: no dynamic dispatch where a package declares //tiger:restrict " +
		"closed-dispatch.",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	declared, ok := restrict.Declared(pass)
	if !ok || !declared.Restriction.ClosedDispatch {
		return nil, nil
	}
	built, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok {
		return nil, nil
	}
	calls := callExprsByLparen(pass)
	for _, fn := range gatherFunctions(built.SrcFuncs) {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				checkInvoke(pass, instr, calls)
			}
		}
	}
	return nil, nil
}

// gatherFunctions collects every function in the package's SSA form,
// declared and closure alike: an index-advancing worklist over AnonFuncs,
// never recursive, mirroring the effects analyzer's own gatherFunctions.
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

// checkInvoke reports TS-K03 when instr is an interface method call whose
// receiver does not devirtualize to exactly one concrete type.
func checkInvoke(pass *analysis.Pass, instr ssa.Instruction, calls map[token.Pos]*ast.CallExpr) {
	call, ok := instr.(ssa.CallInstruction)
	if !ok {
		return
	}
	common := call.Common()
	if !common.IsInvoke() {
		return
	}
	if _, single := devirtualize(common.Value); single {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      common.Pos(),
		Category: "TS-K03",
		Message: "TS-K03: " + callDescription(calls, common) + " is called through interface " +
			interfaceName(common.Value.Type()) + " in a package that declares " +
			"//tiger:restrict closed-dispatch — call the concrete type's method, or drop " +
			"closed-dispatch",
	})
}

// devirtualize walks backward from an interface value through Phi,
// ChangeInterface, and local Store/Load pairs to find the single concrete
// type every path resolves to. Anything else — a parameter, a field, a
// call result, a global, two different concrete types meeting at a Phi —
// leaves ok false.
func devirtualize(root ssa.Value) (types.Type, bool) {
	seen := map[ssa.Value]bool{}
	work := []ssa.Value{root}
	var found types.Type
	for i := 0; i < len(work); i++ {
		value := work[i]
		if seen[value] {
			continue
		}
		seen[value] = true
		resolved := resolveStep(value)
		if !resolved.ok {
			return nil, false
		}
		if resolved.concrete != nil {
			if found != nil && !types.Identical(found, resolved.concrete) {
				return nil, false
			}
			found = resolved.concrete
			continue
		}
		work = append(work, resolved.next...)
	}
	if found == nil {
		return nil, false
	}
	return found, true
}

// step is one backward-resolution move: either more values still to
// resolve, or a concrete type this path terminates on. Ok is false when
// value is a shape this analyzer cannot see through (a parameter, a
// field, a call result, a global, or an allocation with a referrer other
// than the load/store pair this analyzer understands).
type step struct {
	next     []ssa.Value
	concrete types.Type
	ok       bool
}

// resolveStep resolves one SSA value one step backward.
func resolveStep(value ssa.Value) step {
	switch typed := value.(type) {
	case *ssa.MakeInterface:
		return step{concrete: typed.X.Type(), ok: true}
	case *ssa.ChangeInterface:
		return step{next: []ssa.Value{typed.X}, ok: true}
	case *ssa.Phi:
		return step{next: append([]ssa.Value{}, typed.Edges...), ok: true}
	case *ssa.UnOp:
		if typed.Op != token.MUL {
			return step{}
		}
		return loadedValues(typed.X)
	default:
		return step{}
	}
}

// loadedValues resolves a dereference of addr, when addr is a local
// *ssa.Alloc every one of whose referrers is either the load this call
// came from or a Store into it: the values stored are what a load can
// see. Any other referrer — the address escaping to a call, a field
// access, anything this analyzer does not model — is unresolvable.
func loadedValues(addr ssa.Value) step {
	alloc, ok := addr.(*ssa.Alloc)
	if !ok || alloc.Referrers() == nil {
		return step{}
	}
	values := []ssa.Value{}
	for _, referrer := range *alloc.Referrers() {
		switch typed := referrer.(type) {
		case *ssa.Store:
			values = append(values, typed.Val)
		case *ssa.UnOp:
			if typed.Op != token.MUL || typed.X != alloc {
				return step{}
			}
		default:
			return step{}
		}
	}
	return step{next: values, ok: true}
}

// callExprsByLparen maps every call expression in the package to the
// position CallCommon.Pos() reports for it, so a finding can name the
// source-level receiver and method a reader actually wrote.
func callExprsByLparen(pass *analysis.Pass) map[token.Pos]*ast.CallExpr {
	found := map[token.Pos]*ast.CallExpr{}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				found[call.Lparen] = call
			}
			return true
		})
	}
	return found
}

// callDescription renders the call's receiver and method the way the
// source spells them, "s.Write", falling back to the SSA value's own
// name when no matching call expression is found (a shape this analyzer
// does not expect, kept as a known-miss rather than a panic).
func callDescription(calls map[token.Pos]*ast.CallExpr, common *ssa.CallCommon) string {
	call, ok := calls[common.Pos()]
	if !ok {
		return common.Value.Name() + "." + common.Method.Name()
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return common.Value.Name() + "." + common.Method.Name()
	}
	return types.ExprString(selector.X) + "." + selector.Sel.Name
}

// interfaceName renders an interface type the way source names it: a
// named type's own name, or its structural form when it has none.
func interfaceName(t types.Type) string {
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return types.TypeString(t, nil)
}
