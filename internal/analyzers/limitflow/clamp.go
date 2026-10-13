package limitflow

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/counter"
)

// clamped reports whether a statement that dominates expr keeps it below a
// declared maximum: an earlier if that rejects a value above a constant, an
// earlier assignment of min(expr, constant), or an enclosing if that only
// runs when expr is at most a constant.
func (t *tracer) clamped(expr ast.Expr) bool {
	file := t.s.fileOf(expr.Pos())
	if file == nil {
		return false
	}
	want := t.s.spelled(expr)
	path, _ := astutil.PathEnclosingInterval(file, expr.Pos(), expr.End())
	for i := 0; i < len(path)-1; i++ {
		if t.dominated(pathStep{node: path[i], parent: path[i+1]}, want) {
			return true
		}
	}
	return false
}

// pathStep is one node on the way up from an expression and the node that
// holds it.
type pathStep struct {
	node   ast.Node
	parent ast.Node
}

// dominated reports whether the statements before node in its parent, or
// the parent's own condition, keep the expression named want bounded.
func (t *tracer) dominated(step pathStep, want string) bool {
	var list []ast.Stmt
	switch p := step.parent.(type) {
	case *ast.BlockStmt:
		list = p.List
	case *ast.CaseClause:
		list = p.Body
	case *ast.IfStmt:
		return t.guardedBy(p, step.node, want)
	}
	for _, stmt := range list {
		if stmt.Pos() >= step.node.Pos() {
			break
		}
		if t.clampsBefore(stmt, want) {
			return true
		}
	}
	return false
}

// guardedBy reports whether current is the branch of an if that only runs
// while the expression named want is at most a declared maximum.
func (t *tracer) guardedBy(check *ast.IfStmt, current ast.Node, want string) bool {
	if current == check.Body {
		for _, conjunct := range counter.Conjuncts(check.Cond) {
			if t.atMost(conjunct, want, below) {
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
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 || t.s.spelled(assign.Lhs[0]) != want {
		return false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "min" {
		return false
	}
	for _, arg := range call.Args {
		if t.declaredMaximum(arg) {
			return true
		}
	}
	return false
}

// declaredMaximum reports whether expr is a constant, a len or a cap.
func (t *tracer) declaredMaximum(expr ast.Expr) bool {
	if t.s.pass.TypesInfo.Types[expr].Value != nil {
		return true
	}
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := t.s.pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok && (builtin.Name() == "len" || builtin.Name() == "cap")
}

// exceeds reports whether cond, split on ||, has a term true when the
// expression named want is above a declared maximum.
func (t *tracer) exceeds(cond ast.Expr, want string) bool {
	for _, term := range disjuncts(cond) {
		if t.atMost(term, want, above) {
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

// atMost reports whether term compares the expression named want with a
// declared maximum using one of the ordering's operators, in either operand
// order.
func (t *tracer) atMost(term ast.Expr, want string, how ordering) bool {
	binary, ok := ast.Unparen(term).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	left, right := binary.X, binary.Y
	op := binary.Op
	if t.s.spelled(left) != want {
		left, right = right, left
		op = flipped[op]
	}
	if t.s.spelled(left) != want {
		return false
	}
	if op != how.strict && op != how.loose {
		return false
	}
	return t.s.pass.TypesInfo.Types[right].Value != nil
}

// flipped maps a comparison to the one that reads the same with its operands
// swapped.
var flipped = map[token.Token]token.Token{
	token.LSS: token.GTR, token.LEQ: token.GEQ, token.GTR: token.LSS, token.GEQ: token.LEQ,
}

// disjuncts splits expr on top-level ||.
func disjuncts(expr ast.Expr) []ast.Expr {
	pending := []ast.Expr{expr}
	var terms []ast.Expr
	for steps := 0; len(pending) > 0 && steps < disjunctsMax; steps++ {
		last := len(pending) - 1
		next := ast.Unparen(pending[last])
		pending = pending[:last]
		binary, ok := next.(*ast.BinaryExpr)
		if ok && binary.Op == token.LOR {
			pending = append(pending, binary.Y, binary.X)
			continue
		}
		terms = append(terms, next)
	}
	assert.Ok(len(pending) == 0, "disjuncts: condition has more than disjunctsMax terms")
	return terms
}

// disjunctsMax bounds how many nodes the flattening of one condition visits.
const disjunctsMax = 1 << 10

const _ = uint(1<<20 - disjunctsMax)

// unwrapMax bounds how many conversions spelled looks through.
const unwrapMax = 1 << 6

const _ = uint(1<<20 - unwrapMax)

// leaves reports whether body ends by leaving the code that follows the if:
// a return, a break or continue, a panic, or an assignment to want.
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
		return len(last.Lhs) == 1 && t.s.spelled(last.Lhs[0]) == want
	case *ast.ExprStmt:
		call, ok := last.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		ident, ok := call.Fun.(*ast.Ident)
		return ok && ident.Name == "panic"
	}
	return false
}

// spelled prints expr with conversions and parentheses removed, so a clamp
// on req.Limit matches a use written int(req.Limit).
func (s *state) spelled(expr ast.Expr) string {
	for steps := 0; steps < unwrapMax; steps++ {
		expr = ast.Unparen(expr)
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || !s.pass.TypesInfo.Types[call.Fun].IsType() {
			return types.ExprString(expr)
		}
		expr = call.Args[0]
	}
	assert.Fail("spelled: more than unwrapMax nested conversions")
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
