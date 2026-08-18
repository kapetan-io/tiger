// Package contracts enforces TS-V03: a //tiger:requires or //tiger:ensures
// pin blocks the build only when the analyzer can prove the violation by
// constant or nil-literal reasoning; every other obligation stays silent.
//
// TS-A02 already requires every function to assert its own preconditions
// and postconditions at runtime — that is the spec's stated default, and it
// never goes away. What a requires or ensures pin buys on top of that
// default is the chance to catch a violation at compile time instead of at
// the panic. This analyzer resolves predicate refs against the callee's
// parameter names (requires, at each same-package call site) or its result
// names (ensures, at each return statement), substitutes the corresponding
// argument or returned expression, and checks whether pass.TypesInfo can
// prove the substituted predicate false: a literal nil, a compile-time
// integer constant, or a value whose length is knowable at compile time.
// Anything else — a variable, a field path, a bare invariant ID, a
// cross-package callee, a "..." spread call — is a known incompleteness of
// the restricted predicate language, not a finding. It stays silent and
// degrades to the callee's own runtime assertion (TS-A02), which is the
// correct default because the analyzer's incompleteness must never block a
// build.
package contracts

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/pins"
	"github.com/kapetan-io/tiger/internal/directive"
)

const (
	requiresExit = "fix the argument or establish the precondition " +
		"with a dominating check before the call"
	ensuresExit = "fix the returned value so it satisfies the postcondition"
)

// Analyzer enforces TS-V03: a requires or ensures pin fails only when the
// analyzer can prove the violation by constant or nil-literal reasoning.
var Analyzer = &analysis.Analyzer{
	Name: "contracts",
	Doc: "TS-V03: a proven requires or ensures violation blocks; every unproven obligation stays " +
		"silent.",
	Run: run,
}

// pinnedFunc is a same-package function bearing at least one requires pin,
// with the FuncDecl needed to name its parameters at a call site.
type pinnedFunc struct {
	decl     *ast.FuncDecl
	requires []pins.Pin
}

func run(pass *analysis.Pass) (any, error) {
	collected := pins.Collect(pass.Fset, pass.Files)
	funcs := map[types.Object]pinnedFunc{}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			checkEnsures(pass, collected, fn)
			requires := collected.At(fn, "requires")
			if len(requires) == 0 {
				continue
			}
			obj := pass.TypesInfo.Defs[fn.Name]
			if obj == nil {
				continue
			}
			funcs[obj] = pinnedFunc{decl: fn, requires: requires}
		}
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok {
				checkRequires(pass, funcs, call)
			}
			return true
		})
	}
	return nil, nil
}

// checkRequires evaluates every requires pin on call's callee, when the
// callee is a same-package pinned function found in funcs and the call
// passes a fixed argument list — a "..." spread breaks the positional
// correspondence between parameter names and arguments, so it is skipped.
func checkRequires(pass *analysis.Pass, funcs map[types.Object]pinnedFunc, call *ast.CallExpr) {
	if call.Ellipsis != token.NoPos {
		return
	}
	obj := resolveCallee(pass, call)
	if obj == nil {
		return
	}
	callee, ok := funcs[obj]
	if !ok {
		return
	}
	params := fieldNames(callee.decl.Type.Params)
	for _, pin := range callee.requires {
		pred := mustPredicate(pin)
		reason, proven := proveViolation(pass, pred, params, call.Args)
		if !proven {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      call.Pos(),
			Category: "TS-V03",
			Message: "TS-V03: this call violates //tiger:requires " +
				directive.FormatPredicate(pred) + " — " + reason + " — " + requiresExit,
		})
	}
}

// checkEnsures evaluates every ensures pin on fn against each of its
// return statements. It skips past a nested function literal so a
// closure's own return is never attributed to fn.
func checkEnsures(pass *analysis.Pass, collected pins.Set, fn *ast.FuncDecl) {
	ensures := collected.At(fn, "ensures")
	if len(ensures) == 0 {
		return
	}
	if fn.Body == nil {
		return
	}
	results := fieldNames(fn.Type.Results)
	if len(results) == 1 {
		if results[0] == "" {
			results = []string{"result"}
		}
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		ret, ok := node.(*ast.ReturnStmt)
		if ok {
			checkReturn(pass, ensures, results, ret)
		}
		return true
	})
}

// checkReturn evaluates every ensures pin against one return statement. A
// naked return (no Results, valid only with named results) is silent — the
// analyzer would need flow analysis to know the named results' values.
func checkReturn(pass *analysis.Pass, ensures []pins.Pin, results []string, ret *ast.ReturnStmt) {
	if len(ret.Results) == 0 {
		return
	}
	if len(ret.Results) != len(results) {
		return
	}
	for _, pin := range ensures {
		pred := mustPredicate(pin)
		reason, proven := proveViolation(pass, pred, results, ret.Results)
		if !proven {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      ret.Pos(),
			Category: "TS-V03",
			Message: "TS-V03: this return violates //tiger:ensures " +
				directive.FormatPredicate(pred) + " — " + reason + " — " + ensuresExit,
		})
	}
}

// mustPredicate re-parses a pin's argument text. The pins.Set collector only
// collects well-formed pins, so the parse cannot fail here.
func mustPredicate(pin pins.Pin) directive.Predicate {
	pred, err := directive.ParsePredicate(pin.Directive.Args)
	assert.Ok(err == nil, "pins.Set guarantees well-formed predicate args")
	return pred
}

// proveViolation reports whether pred is provably false given names bound
// positionally to exprs — parameters bound to call arguments, or named
// results bound to a return statement's values. The returned bool is false
// whenever the analyzer cannot decide, the silent, unproven case TS-V03
// degrades to a runtime assertion.
func proveViolation(
	pass *analysis.Pass, pred directive.Predicate, names []string, exprs []ast.Expr,
) (string, bool) {
	if pred.Invariant != "" {
		return "", false
	}
	if pred.Left.Kind == directive.AtomNil || pred.Right.Kind == directive.AtomNil {
		return proveNilViolation(pass, pred, names, exprs)
	}
	return proveIntViolation(pass, pred, names, exprs)
}

// proveNilViolation resolves the non-nil predicate atom against a
// substituted argument or return value: a literal nil against a "!="
// requirement, or an address-of expression — provably never nil — against
// a "==" requirement.
func proveNilViolation(
	pass *analysis.Pass, pred directive.Predicate, names []string, exprs []ast.Expr,
) (string, bool) {
	ref := pred.Left
	if ref.Kind == directive.AtomNil {
		ref = pred.Right
	}
	if ref.Kind != directive.AtomRef {
		return "", false
	}
	expr, found := refArg(ref.Ref, names, exprs)
	if !found {
		return "", false
	}
	if pred.Op == "!=" {
		if pass.TypesInfo.Types[expr].IsNil() {
			return "the argument is the literal nil", true
		}
		return "", false
	}
	if isAddressOf(expr) {
		return "the argument " + types.ExprString(expr) + " is never nil", true
	}
	return "", false
}

// isAddressOf reports whether expr takes the address of another value —
// the one syntactic shape the predicate language treats as provably
// non-nil.
func isAddressOf(expr ast.Expr) bool {
	unary, ok := expr.(*ast.UnaryExpr)
	return ok && unary.Op == token.AND
}

// proveIntViolation resolves both predicate atoms to compile-time integer
// values and checks the comparison. Either side may be a parameter/result
// ref, a len() of one, or a same-package named constant; anything else
// leaves the comparison unproven.
func proveIntViolation(
	pass *analysis.Pass, pred directive.Predicate, names []string, exprs []ast.Expr,
) (string, bool) {
	left, leftOK := resolveConstant(pass, pred.Left, names, exprs)
	right, rightOK := resolveConstant(pass, pred.Right, names, exprs)
	if !leftOK || !rightOK {
		return "", false
	}
	if evalComparison(left, pred.Op, right) {
		return "", false
	}
	return fmt.Sprintf("%d %s %d does not hold", left, pred.Op, right), true
}

// resolveConstant resolves one predicate atom to a compile-time integer
// value: a literal from the predicate itself, a substituted argument's
// constant value or length, or a same-package named constant when the ref
// names neither a parameter nor a result.
func resolveConstant(
	pass *analysis.Pass, atom directive.Atom, names []string, exprs []ast.Expr,
) (int64, bool) {
	if atom.Kind == directive.AtomNumber {
		return atom.Int, true
	}
	if atom.Kind == directive.AtomRef {
		expr, found := refArg(atom.Ref, names, exprs)
		if found {
			return exprIntConst(pass, expr)
		}
		return namedIntConst(pass, atom.Ref)
	}
	if atom.Kind == directive.AtomLen {
		expr, found := refArg(atom.Ref, names, exprs)
		if !found {
			return 0, false
		}
		return lengthOf(pass, expr)
	}
	return 0, false
}

// refArg substitutes a bare (undotted) predicate ref for the value bound to
// the same-named entry in names — a call argument or a return value. A
// dotted ref (a field path) or one matching no entry returns ok=false: both
// are unprovable, so the caller degrades to silence.
func refArg(ref string, names []string, exprs []ast.Expr) (ast.Expr, bool) {
	if strings.Contains(ref, ".") {
		return nil, false
	}
	for i, name := range names {
		if name != ref {
			continue
		}
		if i >= len(exprs) {
			return nil, false
		}
		return exprs[i], true
	}
	return nil, false
}

// exprIntConst reads expr's compile-time integer value, when the type
// checker recorded one — true for both integer literals and named integer
// constants used as arguments.
func exprIntConst(pass *analysis.Pass, expr ast.Expr) (int64, bool) {
	folded := pass.TypesInfo.Types[expr].Value
	if folded == nil {
		return 0, false
	}
	if folded.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(folded)
}

// namedIntConst resolves ref as a package-level named integer constant —
// the "HeaderSizeBytes" half of len(target) == HeaderSizeBytes, which names
// no parameter of the callee.
func namedIntConst(pass *analysis.Pass, ref string) (int64, bool) {
	obj := pass.Pkg.Scope().Lookup(ref)
	named, ok := obj.(*types.Const)
	if !ok {
		return 0, false
	}
	if named.Val().Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(named.Val())
}

// lengthOf resolves a compile-time length for expr: a string constant's
// length, a composite literal's element count, or an array type's length.
func lengthOf(pass *analysis.Pass, expr ast.Expr) (int64, bool) {
	tv := pass.TypesInfo.Types[expr]
	if tv.Value != nil && tv.Value.Kind() == constant.String {
		return int64(len(constant.StringVal(tv.Value))), true
	}
	if lit, ok := expr.(*ast.CompositeLit); ok {
		return int64(len(lit.Elts)), true
	}
	if tv.Type == nil {
		return 0, false
	}
	array, ok := tv.Type.Underlying().(*types.Array)
	if !ok {
		return 0, false
	}
	return array.Len(), true
}

// resolveCallee resolves call's callee to the types.Object its declaration
// was checked against, for both a plain call and a method call.
func resolveCallee(pass *analysis.Pass, call *ast.CallExpr) types.Object {
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return pass.TypesInfo.Uses[ident]
	}
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		return pass.TypesInfo.Uses[selector.Sel]
	}
	return nil
}

// fieldNames flattens a parameter or result list into positional names —
// "" for an unnamed field, which never matches a predicate ref.
func fieldNames(fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	var names []string
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			names = append(names, "")
			continue
		}
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

// evalComparison applies pred's comparison operator to two resolved
// integer values. The op parameter is always one of the six the predicate
// grammar admits.
func evalComparison(left int64, op string, right int64) bool {
	if op == "==" {
		return left == right
	}
	if op == "!=" {
		return left != right
	}
	if op == "<" {
		return left < right
	}
	if op == "<=" {
		return left <= right
	}
	if op == ">" {
		return left > right
	}
	assert.Ok(op == ">=", "predicate operator outside the closed comparison set")
	return left >= right
}
