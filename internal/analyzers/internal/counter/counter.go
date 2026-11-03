package counter

import (
	"go/ast"
	"go/token"
	"go/types"
	"math/big"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/condition"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/intrange"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
)

type Term struct {
	Counter   *types.Var
	Limit     ast.Expr
	Inclusive bool
	Size      int64
	Down      bool
}

type step struct {
	counter *types.Var
	size    int64
	down    bool
}

func Terms(scope mutation.Scope, loop *ast.ForStmt) []Term {
	if loop.Cond == nil {
		return nil
	}
	moved, ok := advance(scope.Info(), loop.Post)
	if !ok {
		return nil
	}
	if disturbed(scope, loop, moved.counter) {
		return nil
	}
	var terms []Term
	for _, conjunct := range condition.Conjuncts(loop.Cond) {
		term, sound := compare(scope, loop, conjunct, moved)
		if sound && Reaches(scope, term) {
			terms = append(terms, term)
		}
	}
	return terms
}

var addsOrSubtracts = map[token.Token]bool{token.ADD_ASSIGN: true, token.SUB_ASSIGN: true}

func Counted(scope mutation.Scope, loop *ast.ForStmt) bool {
	if loop.Cond == nil {
		return false
	}
	var target ast.Expr
	switch post := loop.Post.(type) {
	case *ast.IncDecStmt:
		target = post.X
	case *ast.AssignStmt:
		if len(post.Lhs) == 1 && addsOrSubtracts[post.Tok] {
			target = post.Lhs[0]
		}
	}
	if target == nil {
		return false
	}
	counted, ok := integerVar(scope.Info(), target)
	return ok && scope.Mentions(loop.Cond, counted)
}

func advance(info *types.Info, post ast.Stmt) (step, bool) {
	switch stmt := post.(type) {
	case *ast.IncDecStmt:
		counter, ok := integerVar(info, stmt.X)
		return step{counter: counter, size: 1, down: stmt.Tok == token.DEC}, ok
	case *ast.AssignStmt:
		if len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
			return step{}, false
		}
		if stmt.Tok != token.ADD_ASSIGN && stmt.Tok != token.SUB_ASSIGN {
			return step{}, false
		}
		counter, ok := integerVar(info, stmt.Lhs[0])
		size, constantStep := intrange.SmallConstant(info, stmt.Rhs[0])
		if !ok || !constantStep || size < 1 {
			return step{}, false
		}
		return step{counter: counter, size: size, down: stmt.Tok == token.SUB_ASSIGN}, true
	}
	return step{}, false
}

func integerVar(info *types.Info, expr ast.Expr) (*types.Var, bool) {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return nil, false
	}
	v, ok := info.ObjectOf(ident).(*types.Var)
	if !ok || v.IsField() {
		return nil, false
	}
	return v, intrange.IsInteger(v.Type())
}

func disturbed(scope mutation.Scope, loop *ast.ForStmt, counter *types.Var) bool {
	if len(scope.Writes(loop.Body, counter)) > 0 {
		return true
	}
	return scope.Escapes(counter)
}

func compare(
	scope mutation.Scope, loop *ast.ForStmt, conjunct ast.Expr, moved step,
) (Term, bool) {
	binary, ok := conjunct.(*ast.BinaryExpr)
	if !ok {
		return Term{}, false
	}
	op, limit := binary.Op, binary.Y
	switch {
	case scope.IsVar(binary.X, moved.counter):
	case scope.IsVar(binary.Y, moved.counter):
		op, limit = flipped(binary.Op), binary.X
	default:
		return Term{}, false
	}
	if scope.Mentions(limit, moved.counter) {
		return Term{}, false
	}
	toward := op == token.LSS || op == token.LEQ
	if moved.down {
		toward = op == token.GTR || op == token.GEQ
	}
	if !toward || !stable(scope, loop, limit) {
		return Term{}, false
	}
	inclusive := op == token.LEQ || op == token.GEQ
	return Term{
		Counter: moved.counter, Limit: limit, Inclusive: inclusive,
		Size: moved.size, Down: moved.down,
	}, true
}

func flipped(op token.Token) token.Token {
	mirrored, _ := condition.Mirror(op)
	return mirrored
}

func stable(scope mutation.Scope, loop *ast.ForStmt, limit ast.Expr) bool {
	holds := true
	ast.Inspect(limit, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.CallExpr:
			holds = holds && staysPut(scope.Info(), expr)
		case *ast.FuncLit:
			holds = false
		case *ast.StarExpr:
			holds = holds && !writesText(loop.Body, expr)
		case *ast.IndexExpr:
			holds = holds && !writesText(loop.Body, expr)
		case *ast.UnaryExpr:
			holds = holds && valueOperators[expr.Op]
		case *ast.Ident:
			holds = holds && variableHolds(scope, loop, expr)
		case *ast.SelectorExpr:
			holds = holds && fieldHolds(scope, loop, expr)
		}
		return holds
	})
	return holds
}

var limitBuiltins = map[string]bool{"len": true, "cap": true, "min": true, "max": true}

var valueOperators = map[token.Token]bool{token.SUB: true, token.ADD: true, token.XOR: true}

func writesText(body *ast.BlockStmt, expr ast.Expr) bool {
	want := types.ExprString(expr)
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return !found
		}
		for _, lhs := range assign.Lhs {
			if types.ExprString(lhs) == want {
				found = true
			}
		}
		return !found
	})
	return found
}

func staysPut(info *types.Info, call *ast.CallExpr) bool {
	if info.Types[call.Fun].IsType() {
		return true
	}
	var callee types.Object
	switch fun := condition.Uninstantiated(call.Fun).(type) {
	case *ast.Ident:
		callee = info.Uses[fun]
	case *ast.SelectorExpr:
		callee = info.Uses[fun.Sel]
	}
	switch named := callee.(type) {
	case *types.Builtin:
		return limitBuiltins[named.Name()]
	case *types.Func:
		return true
	}
	return false
}

func variableHolds(scope mutation.Scope, loop *ast.ForStmt, ident *ast.Ident) bool {
	v, ok := scope.Info().ObjectOf(ident).(*types.Var)
	if !ok || v.IsField() {
		return true
	}
	if len(scope.Writes(loop.Body, v)) > 0 {
		return false
	}
	if growable(v.Type()) && scope.PassedToCall(loop.Body, v) {
		return false
	}
	return !scope.Escapes(v)
}

func growable(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Map, *types.Chan:
		return true
	}
	return false
}

func fieldHolds(scope mutation.Scope, loop *ast.ForStmt, selector *ast.SelectorExpr) bool {
	selection := scope.Info().Selections[selector]
	if selection == nil || selection.Kind() != types.FieldVal {
		return true
	}
	field, ok := selection.Obj().(*types.Var)
	if !ok {
		return false
	}
	if len(scope.FieldWrites(loop.Body, field)) > 0 {
		return false
	}
	if scope.FieldEscapes(field) {
		return false
	}
	return !scope.ReceivesOwner(loop.Body, selection.Recv())
}

func trusted(typed intrange.Span, term Term) bool {
	if term.Size == 1 && !term.Inclusive {
		return true
	}
	wide := typed.OutOfReach()
	if term.Down {
		return wide && typed.Low.Sign() < 0
	}
	return wide
}

func Reaches(scope mutation.Scope, term Term) bool {
	typed, ok := intrange.Of(term.Counter.Type(), scope.Sizes())
	if !ok {
		return false
	}
	limit, known := scope.ConstantRange(term.Limit)
	if !known {
		return trusted(typed, term)
	}
	size := big.NewInt(term.Size)
	if term.Down {
		lowest := new(big.Int).Sub(limit.Low, size)
		if !term.Inclusive {
			lowest.Add(lowest, big.NewInt(1))
		}
		return lowest.Cmp(typed.Low) >= 0
	}
	highest := new(big.Int).Add(limit.High, size)
	if !term.Inclusive {
		highest.Sub(highest, big.NewInt(1))
	}
	return highest.Cmp(typed.High) <= 0
}
