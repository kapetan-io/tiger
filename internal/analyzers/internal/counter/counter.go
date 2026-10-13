// Package counter decides whether a for loop's counter is a sound bound,
// shared by boundedloop (TS-S02) and variant (TS-V01).
//
// A counter is a bound only when its comparison is a top-level && term of
// the loop's condition, pairs < or <= with ++ or += (or > or >= with -- or
// -=, never !=), nothing in the body writes the counter or its limit, and
// the counter reaches the limit before it wraps around the type's range.
// The limit may call only len, cap, min, max, conversions and declared
// functions or methods; a function value could return something larger on
// every pass. A declared getter such as tuple.Len() is trusted, so a body
// that grows the receiver through a method call goes unseen.
package counter

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/big"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/intrange"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
)

// Term is a counter's march toward a limit: the comparison that bounds it,
// whether the comparison includes the limit itself, and the constant step
// the counter takes on each pass toward it.
type Term struct {
	Counter   *types.Var
	Limit     ast.Expr
	Inclusive bool
	Size      int64
	Down      bool
}

// step is how a Post clause moves its counter on every pass.
type step struct {
	counter *types.Var
	size    int64
	down    bool
}

// Terms returns the comparisons in loop's condition that bound the loop
// soundly. It is empty when the loop has no counter, the counter is
// disturbed, or no comparison qualifies.
func Terms(scope mutation.Scope, sizes types.Sizes, loop *ast.ForStmt) []Term {
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
	for _, conjunct := range Conjuncts(loop.Cond) {
		term, sound := compare(scope, loop, conjunct, moved)
		if sound && Reaches(scope, sizes, term) {
			terms = append(terms, term)
		}
	}
	return terms
}

// conjunctsMax bounds how many nodes the flattening of one condition visits.
const conjunctsMax = 1 << 12

const _ = uint(1<<20 - conjunctsMax)

// Conjuncts splits expr on top-level && into its terms, looking through
// parentheses but not through ! or ||.
func Conjuncts(expr ast.Expr) []ast.Expr {
	pending := []ast.Expr{expr}
	var terms []ast.Expr
	for steps := 0; len(pending) > 0 && steps < conjunctsMax; steps++ {
		last := len(pending) - 1
		next := ast.Unparen(pending[last])
		pending = pending[:last]
		binary, ok := next.(*ast.BinaryExpr)
		if ok && binary.Op == token.LAND {
			pending = append(pending, binary.Y, binary.X)
			continue
		}
		terms = append(terms, next)
	}
	assert.Ok(len(pending) == 0, "condition has more than conjunctsMax terms")
	return terms
}

// advance reads the counter and the constant step a Post clause applies.
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
		size, constantStep := smallConstant(info, stmt.Rhs[0])
		if !ok || !constantStep || size < 1 {
			return step{}, false
		}
		return step{counter: counter, size: size, down: stmt.Tok == token.SUB_ASSIGN}, true
	}
	return step{}, false
}

// integerVar resolves expr to a local integer variable.
func integerVar(info *types.Info, expr ast.Expr) (*types.Var, bool) {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return nil, false
	}
	v, ok := info.ObjectOf(ident).(*types.Var)
	if !ok || v.IsField() {
		return nil, false
	}
	basic, ok := v.Type().Underlying().(*types.Basic)
	return v, ok && basic.Info()&types.IsInteger != 0
}

// smallConstant returns expr's value when it is an integer constant that
// fits in an int64.
func smallConstant(info *types.Info, expr ast.Expr) (int64, bool) {
	value := info.Types[expr].Value
	if value == nil || value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(value)
}

// disturbed reports whether the body, or something outside it, writes the
// counter.
func disturbed(scope mutation.Scope, loop *ast.ForStmt, counter *types.Var) bool {
	if len(scope.Writes(loop.Body, counter)) > 0 {
		return true
	}
	return scope.Escapes(counter)
}

// compare reads one conjunct as counter-versus-limit in the direction the
// step moves, and checks the limit holds still while the loop runs.
func compare(
	scope mutation.Scope, loop *ast.ForStmt, conjunct ast.Expr, moved step,
) (Term, bool) {
	binary, ok := conjunct.(*ast.BinaryExpr)
	if !ok {
		return Term{}, false
	}
	op, limit := binary.Op, binary.Y
	switch {
	case isVar(scope.Info(), binary.X, moved.counter):
	case isVar(scope.Info(), binary.Y, moved.counter):
		op, limit = mirrored[binary.Op], binary.X
	default:
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

// mirrored maps a comparison to the one that reads the same with its
// operands swapped.
var mirrored = map[token.Token]token.Token{
	token.LSS: token.GTR, token.LEQ: token.GEQ, token.GTR: token.LSS, token.GEQ: token.LEQ,
}

func isVar(info *types.Info, expr ast.Expr, v *types.Var) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && info.ObjectOf(ident) == v
}

// stable reports whether nothing in the loop can change limit's value.
func stable(scope mutation.Scope, loop *ast.ForStmt, limit ast.Expr) bool {
	holds := true
	ast.Inspect(limit, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.CallExpr:
			holds = holds && staysPut(scope.Info(), expr)
		case *ast.FuncLit:
			holds = false
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

// limitBuiltins are the builtins whose result a loop body cannot grow from
// the outside.
var limitBuiltins = map[string]bool{"len": true, "cap": true, "min": true, "max": true}

// valueOperators are the unary operators that leave a limit a plain value.
var valueOperators = map[token.Token]bool{token.SUB: true, token.ADD: true, token.XOR: true}

// staysPut reports whether a call in a limit is trusted to hold still: a
// conversion, a builtin that reads its operands, or a call to a declared
// function or method such as tuple.Len(). The operands of the call are
// judged with the rest of the limit; a function value is not trusted,
// because nothing says what it returns next.
func staysPut(info *types.Info, call *ast.CallExpr) bool {
	if info.Types[call.Fun].IsType() {
		return true
	}
	var callee types.Object
	switch fun := ast.Unparen(call.Fun).(type) {
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

// variableHolds reports whether the variable ident names is unwritten by
// the loop and by anything that outlives it.
func variableHolds(scope mutation.Scope, loop *ast.ForStmt, ident *ast.Ident) bool {
	v, ok := scope.Info().ObjectOf(ident).(*types.Var)
	if !ok || v.IsField() {
		return true
	}
	if len(scope.Writes(loop.Body, v)) > 0 {
		return false
	}
	return !scope.Escapes(v)
}

// fieldHolds reports whether the field a selector reads is unwritten by the
// loop, by anything that outlives it, and by any call the body makes with
// the struct that owns it.
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

// Reaches reports whether the counter arrives at the limit before it wraps
// around the end of its type. A limit the analyzer cannot reduce to a
// constant is trusted, since only a constant at the type's edge is a proof
// of the wrap.
func Reaches(scope mutation.Scope, sizes types.Sizes, term Term) bool {
	typed, ok := intrange.Of(term.Counter.Type(), sizes)
	if !ok {
		return false
	}
	limit, known := scope.ConstantRange(term.Limit)
	if !known {
		return true
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
