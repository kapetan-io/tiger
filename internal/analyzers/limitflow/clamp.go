package limitflow

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/condition"
)

// clamped reports whether a statement that dominates expr keeps it below a
// declared maximum: an earlier if that rejects a value above a constant, an
// earlier assignment of min(expr, constant), or an enclosing if that only
// runs when expr is at most a constant.
func (t *tracer) clamped(expr ast.Expr) bool {
	file := t.s.fileOf(expr.Pos())
	if file == nil || !t.simple(expr) {
		return false
	}
	want := t.s.identity(expr)
	path, _ := astutil.PathEnclosingInterval(file, expr.Pos(), expr.End())
	for _, enclosing := range path {
		loop, isLoop := enclosing.(ast.Stmt)
		if !isLoop || !isLoopStmt(enclosing) {
			continue
		}
		if t.writesTo(loop, expr) {
			return false
		}
	}
	for i := 0; i < len(path)-1; i++ {
		if t.dominated(pathStep{node: path[i], parent: path[i+1]}, expr, want) {
			return true
		}
	}
	return false
}

// isLoopStmt reports whether node is a for or range statement.
func isLoopStmt(node ast.Node) bool {
	switch node.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return true
	}
	return false
}

// simple reports whether expr is a variable, or a chain of fields over one,
// which holds still between two reads unless something writes it. A call or an
// index can answer differently each time, so a clamp on one says nothing about
// the next.
func (t *tracer) simple(expr ast.Expr) bool {
	expr = t.s.withoutConversions(expr)
	for steps := 0; steps < unwrapMax; steps++ {
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			_, isIdent := expr.(*ast.Ident)
			return isIdent
		}
		expr = ast.Unparen(selector.X)
	}
	assert.Fail("simple: more than unwrapMax nested selectors")
	return false
}

// pathStep is one node on the way up from an expression and the node that
// holds it.
type pathStep struct {
	node   ast.Node
	parent ast.Node
}

// dominated reports whether the statements before node in its parent, or
// the parent's own condition, keep the expression named want bounded. A write
// to the expression after the clamp undoes it.
func (t *tracer) dominated(step pathStep, target ast.Expr, want string) bool {
	var list []ast.Stmt
	switch p := step.parent.(type) {
	case *ast.BlockStmt:
		list = p.List
	case *ast.CaseClause:
		list = p.Body
	case *ast.IfStmt:
		return t.guardedBy(p, step.node, want)
	}
	clamped := false
	for _, stmt := range list {
		if stmt.Pos() >= step.node.Pos() {
			break
		}
		if t.clampsBefore(stmt, want) {
			clamped = true
		} else if t.writesTo(stmt, target) {
			clamped = false
		}
	}
	return clamped
}

// writesTo reports whether stmt changes the variable or field chain expr
// names: it assigns any field along the chain, assigns the variable or what a
// pointer to it points at, or hands a value of a holding struct to a call.
func (t *tracer) writesTo(stmt ast.Stmt, expr ast.Expr) bool {
	expr = t.s.withoutConversions(expr)
	for steps := 0; steps < unwrapMax; steps++ {
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return t.writesVariable(stmt, expr)
		}
		if t.writesField(stmt, selector) {
			return true
		}
		expr = ast.Unparen(selector.X)
	}
	assert.Fail("writesTo: more than unwrapMax nested selectors")
	return false
}

// writesField reports whether stmt assigns the field a selector reads, or
// passes a call a value of the struct that holds it.
func (t *tracer) writesField(stmt ast.Stmt, selector *ast.SelectorExpr) bool {
	selection := t.s.pass.TypesInfo.Selections[selector]
	if selection == nil || selection.Kind() != types.FieldVal {
		return false
	}
	field, ok := selection.Obj().(*types.Var)
	if !ok {
		return false
	}
	if len(t.fn.scope.FieldWrites(stmt, field)) > 0 {
		return true
	}
	return t.fn.scope.ReceivesOwner(stmt, selection.Recv())
}

// writesVariable reports whether stmt assigns the variable expr names or
// what it points at.
func (t *tracer) writesVariable(stmt ast.Stmt, expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := t.s.pass.TypesInfo.ObjectOf(ident).(*types.Var)
	if !ok {
		return false
	}
	if len(t.fn.scope.Writes(stmt, v)) > 0 {
		return true
	}
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		assign, isAssign := n.(*ast.AssignStmt)
		if !isAssign {
			return !found
		}
		for _, lhs := range assign.Lhs {
			star, isStar := lhs.(*ast.StarExpr)
			if isStar && t.fn.scope.IsVar(star.X, v) {
				found = true
			}
		}
		return !found
	})
	return found
}

// guardedBy reports whether current is the branch of an if that only runs
// while the expression named want is at most a declared maximum.
func (t *tracer) guardedBy(check *ast.IfStmt, current ast.Node, want string) bool {
	if current == check.Body {
		for _, conjunct := range condition.Conjuncts(check.Cond) {
			if t.comparesWithMaximum(conjunct, want, below) {
				return true
			}
		}
	}
	if current == check.Else {
		return t.exceeds(check.Cond, want)
	}
	return false
}

// clampsBefore reports whether stmt, which runs before the use, bounds the
// expression named want.
func (t *tracer) clampsBefore(stmt ast.Stmt, want string) bool {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return t.assignsMin(s, want)
	case *ast.IfStmt:
		return t.exceeds(s.Cond, want) && t.leaves(s.Body, want)
	}
	return false
}

// assignsMin reports whether assign is want = min(want, bound) with a bound
// that is a constant or a length.
func (t *tracer) assignsMin(assign *ast.AssignStmt, want string) bool {
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 || t.s.identity(assign.Lhs[0]) != want {
		return false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	if !t.isBuiltin(call, "min") {
		return false
	}
	for _, arg := range call.Args {
		if t.declaredMaximum(arg) {
			return true
		}
	}
	return false
}

// isBuiltin reports whether call is to the named predeclared function, not a
// function of the same name declared in the package.
func (t *tracer) isBuiltin(call *ast.CallExpr, name string) bool {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := t.s.pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok && builtin.Name() == name
}

// declaredMaximum reports whether expr is a constant, a len or a cap.
func (t *tracer) declaredMaximum(expr ast.Expr) bool {
	if t.s.pass.TypesInfo.Types[expr].Value != nil {
		return true
	}
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	return ok && (t.isBuiltin(call, "len") || t.isBuiltin(call, "cap"))
}

// exceeds reports whether cond, split on ||, has a term true when the
// expression named want is above a declared maximum.
func (t *tracer) exceeds(cond ast.Expr, want string) bool {
	for _, term := range condition.Disjuncts(cond) {
		if t.comparesWithMaximum(term, want, above) {
			return true
		}
	}
	return false
}

// ordering is a pair of comparison operators, one that excludes the maximum
// and one that includes it.
type ordering struct {
	strict token.Token
	loose  token.Token
}

var (
	// below is true while a value is under, or at, its maximum.
	below = ordering{strict: token.LSS, loose: token.LEQ}
	// above is true once a value is over, or at, its maximum.
	above = ordering{strict: token.GTR, loose: token.GEQ}
)

// comparesWithMaximum reports whether term compares the expression named want with a
// declared maximum using one of the ordering's operators, in either operand
// order.
func (t *tracer) comparesWithMaximum(term ast.Expr, want string, how ordering) bool {
	binary, ok := ast.Unparen(term).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	left, right := binary.X, binary.Y
	op := binary.Op
	if t.s.identity(left) != want {
		left, right = right, left
		op, _ = condition.Mirror(op)
	}
	if t.s.identity(left) != want {
		return false
	}
	if op != how.strict && op != how.loose {
		return false
	}
	return t.s.pass.TypesInfo.Types[right].Value != nil
}

// unwrapMax bounds how many nested conversions or selectors withoutConversions
// and qualified peel off one expression. Real code nests a handful.
const unwrapMax = 1 << 6

const _ = uint(1<<20 - unwrapMax)

// leaves reports whether body ends by leaving the code that follows the if:
// a return, a break or continue, a panic, or an assignment of a declared
// maximum to want.
func (t *tracer) leaves(body *ast.BlockStmt, want string) bool {
	if len(body.List) == 0 {
		return false
	}
	switch last := body.List[len(body.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return true
	case *ast.AssignStmt:
		if len(last.Lhs) != 1 || len(last.Rhs) != 1 || t.s.identity(last.Lhs[0]) != want {
			return false
		}
		return t.declaredMaximum(last.Rhs[0]) || t.clampsItself(last.Rhs[0])
	case *ast.ExprStmt:
		call, ok := last.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		return t.isBuiltin(call, "panic")
	}
	return false
}

// identity prints expr with conversions and parentheses removed and its
// variable named by where it is declared, so a shadow that is spelled the
// same is a different expression and a clamp on req.Limit matches a use
// written int(req.Limit).
func (s *state) identity(expr ast.Expr) string {
	return s.qualified(s.withoutConversions(expr))
}

// withoutConversions removes parentheses and conversions around expr.
func (s *state) withoutConversions(expr ast.Expr) ast.Expr {
	for steps := 0; steps < unwrapMax; steps++ {
		expr = ast.Unparen(expr)
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || !s.pass.TypesInfo.Types[call.Fun].IsType() {
			return expr
		}
		expr = call.Args[0]
	}
	assert.Fail("withoutConversions: more than unwrapMax nested conversions")
	return expr
}

// qualified prints a chain of field selectors over a variable, naming the
// variable by its declaration, and any other expression as written.
func (s *state) qualified(expr ast.Expr) string {
	var fields string
	for steps := 0; steps < unwrapMax; steps++ {
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			text := types.ExprString(expr)
			if ident, isIdent := expr.(*ast.Ident); isIdent {
				if object := s.pass.TypesInfo.ObjectOf(ident); object != nil {
					text = fmt.Sprintf("%s@%d", ident.Name, object.Pos())
				}
			}
			return text + fields
		}
		fields = "." + selector.Sel.Name + fields
		expr = ast.Unparen(selector.X)
	}
	assert.Fail("qualified: more than unwrapMax nested selectors")
	return ""
}

// fileOf returns the file holding pos.
func (s *state) fileOf(pos token.Pos) *ast.File {
	for _, file := range s.pass.Files {
		if file.Pos() <= pos && pos <= file.End() {
			return file
		}
	}
	return nil
}
