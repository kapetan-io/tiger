// Package boundedloop enforces TS-S02 and TS-S03: every loop has an upper
// bound, and an unbounded event loop selects on ctx.Done() or a recognized
// shutdown channel.
//
// A RangeStmt over a slice, array, map, string, integer, or range-over-func
// iterator is bounded by its own syntax. A RangeStmt over a channel is not:
// termination depends on another goroutine closing it, so it is a TS-S02
// finding. A ForStmt with no Cond ("for {}") is either the TS-S03 event-loop
// shape — a direct SelectStmt with a case receiving from ctx.Done() or a
// recognized shutdown channel — or a TS-S02 finding.
//
// A ForStmt with a Cond is bounded when its Post advances a variable that
// appears in Cond (a counter), or when Cond — recursing through parens, !,
// && (either operand bounded), and || (every operand bounded) — reduces to
// one of: a comparison against a constant, a len(), or a cap(); a
// monotone-counter comparison against zero (x != 0, with every assignment
// to x inside the loop — matched by object identity, so a shadow is a
// different variable — provably a right-shift or a division by a constant
// greater than one, and at least one of them guaranteed every iteration:
// the Post clause, or a direct body statement no continue or goto can
// skip); or a tuple-post reversal (i < j / i <= j, with the Post moving i
// up and j down by the same constant step, closing the gap).
// A FuncLit is a frame boundary for the monotone-counter and tuple-post
// direction scans: an assignment inside a closure counts neither for nor
// against the proof, matching deferdistance's and declusedistance's
// treatment of the same shape.
//
// Any other Cond is a TS-S02 finding, unless the loop matches the cursor
// shape — Cond is a boolean method call on a plain identifier (it.Valid(),
// rows.Next()) — and carries a well-formed //tiger:batched directive: the
// store's finiteness is a fact of the world the analyzer cannot derive, so
// the waiver is shape-gated rather than a blanket suppression.
package boundedloop

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/attach"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/shutdown"
)

const (
	msgUnbounded = "TS-S02: this loop's bound cannot be derived from a constant, a len, or a " +
		"counter — give it an explicit cap with an assert or error on exhaustion, or use the " +
		"ctx.Done() event-loop shape (TS-S03)"
	msgNoSelect = "TS-S02: this loop has no condition and no ctx.Done() select — give it an " +
		"explicit cap with an assert or error on exhaustion, or use the ctx.Done() event-loop " +
		"shape (TS-S03)"
	msgChannelRange = "TS-S02: ranging over a channel terminates only when another goroutine " +
		"closes it — give it an explicit counter cap with an assert or error on exhaustion, or " +
		"use the ctx.Done() event-loop shape (TS-S03)"
	msgSelectNoDone = "TS-S03: this unbounded event loop's select has no case receiving from " +
		"ctx.Done() or a recognized shutdown channel — add a case <-ctx.Done(): return, or a " +
		"case on a struct{}-typed or shutdown-named channel, so the loop has a termination path"
)

// Analyzer enforces TS-S02 and TS-S03: every loop has an upper bound, or the
// event-loop shape.
var Analyzer = &analysis.Analyzer{
	Name: "boundedloop",
	Doc:  "TS-S02 and TS-S03: every loop has an upper bound or the event-loop shape.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch loop := node.(type) {
			case *ast.RangeStmt:
				checkRange(pass, loop)
			case *ast.ForStmt:
				checkFor(pass, file, loop)
			}
			return true
		})
	}
	return nil, nil
}

// checkRange fires TS-S02 for a range over a channel; every other range
// target is bounded by its own syntax.
func checkRange(pass *analysis.Pass, loop *ast.RangeStmt) {
	rangeType := pass.TypesInfo.TypeOf(loop.X)
	if rangeType == nil {
		return
	}
	if _, isChannel := rangeType.Underlying().(*types.Chan); isChannel {
		pass.Report(analysis.Diagnostic{
			Pos:      loop.Pos(),
			Category: "TS-S02",
			Message:  msgChannelRange,
		})
	}
}

// checkFor classifies a for statement: the event-loop shape (no Cond), a
// bounded three-clause loop, a waived cursor-shaped loop, or a TS-S02
// finding.
func checkFor(pass *analysis.Pass, file *ast.File, loop *ast.ForStmt) {
	if loop.Cond == nil {
		checkEventLoop(pass, loop)
		return
	}
	if boundedCond(pass, loop) {
		return
	}
	if cursorShape(pass, loop) {
		if _, ok := attach.Directive(pass.Fset, file, loop, "batched"); ok {
			return
		}
	}
	pass.Report(analysis.Diagnostic{
		Pos:      loop.Cond.Pos(),
		Category: "TS-S02",
		Message:  msgUnbounded,
	})
}

// checkEventLoop handles a for{} with no condition: the TS-S03 shape (a
// direct select with a ctx.Done() or shutdown-channel case), a select
// missing that case (TS-S03), or no select at all (TS-S02). A select with
// a default clause is non-blocking — a drain loop, not an event loop
// waiting on the outside world — so it needs no shutdown case, matching
// TS-C05's treatment of the same shape.
func checkEventLoop(pass *analysis.Pass, loop *ast.ForStmt) {
	sel := directSelect(loop.Body)
	if sel == nil {
		pass.Report(analysis.Diagnostic{
			Pos:      loop.Pos(),
			Category: "TS-S02",
			Message:  msgNoSelect,
		})
		return
	}
	if selectHasDefault(sel) {
		return
	}
	if !selectHasTerminationCase(pass, sel) {
		pass.Report(analysis.Diagnostic{
			Pos:      sel.Pos(),
			Category: "TS-S03",
			Message:  msgSelectNoDone,
		})
	}
}

// selectHasDefault reports whether the select carries a default clause,
// which makes it non-blocking.
func selectHasDefault(sel *ast.SelectStmt) bool {
	for _, clause := range sel.Body.List {
		if comm, ok := clause.(*ast.CommClause); ok && comm.Comm == nil {
			return true
		}
	}
	return false
}

// directSelect returns the first SelectStmt among the block's direct
// statements, or nil if none is present.
func directSelect(block *ast.BlockStmt) *ast.SelectStmt {
	for _, stmt := range block.List {
		if sel, ok := stmt.(*ast.SelectStmt); ok {
			return sel
		}
	}
	return nil
}

// selectHasTerminationCase reports whether the select has a comm case
// receiving from a call to a method named Done, or from a recognized
// shutdown channel (see internal/analyzers/internal/shutdown).
func selectHasTerminationCase(pass *analysis.Pass, sel *ast.SelectStmt) bool {
	for _, clause := range sel.Body.List {
		comm, ok := clause.(*ast.CommClause)
		if ok && receivesFromTermination(pass, comm.Comm) {
			return true
		}
	}
	return false
}

// receivesFromTermination reports whether comm receives from a call to a
// method named Done, or from a recognized shutdown channel, covering both
// "<-ch" and "v, ok := <-ch" forms. A nil comm is a select's default case.
func receivesFromTermination(pass *analysis.Pass, comm ast.Stmt) bool {
	var recv ast.Expr
	switch clause := comm.(type) {
	case *ast.ExprStmt:
		recv = clause.X
	case *ast.AssignStmt:
		if len(clause.Rhs) == 1 {
			recv = clause.Rhs[0]
		}
	}
	unary, ok := recv.(*ast.UnaryExpr)
	if !ok || unary.Op != token.ARROW {
		return false
	}
	return doneCall(unary.X) || shutdown.Channel(pass.TypesInfo, unary.X)
}

// doneCall reports whether expr is a call whose callee is a method named
// Done, the ctx.Done() shape.
func doneCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Done"
}

// boundedCond reports whether loop's Cond/Post pair proves a bound: a
// counter advanced in Post that appears anywhere in Cond (the loop-level
// acceptance, unchanged since wave 1), or a Cond that reduces — through
// &&, ||, parens, and ! — to a leaf comparison boundedCond can prove.
func boundedCond(pass *analysis.Pass, loop *ast.ForStmt) bool {
	if counter := postCounter(loop.Post); counter != "" && identifierAppears(loop.Cond, counter) {
		return true
	}
	return condBounded(pass, loop, loop.Cond, plainContext)
}

// polarity records whether an enclosing negation operator has inverted
// the boolean context a condition is judged in; De Morgan flips the
// &&/|| rules under negation.
type polarity bool

const (
	plainContext   polarity = false
	negatedContext polarity = true
)

// counterSign records whether a monotone counter's type is unsigned — the
// only sign for which a right shift provably reaches zero.
type counterSign bool

const (
	signedCounter   counterSign = false
	unsignedCounter counterSign = true
)

// condBounded recurses through the boolean structure of a condition: && is
// bounded if either operand is bounded (the loop exits when the bounded
// side goes false); || is bounded only if every operand is bounded (either
// alone going false still lets the loop run on the other side). Negation
// flips the two rules by De Morgan — !(A && B) is !A || !B, so under
// negation the &&-either rule would accept a loop the bounded arm cannot
// stop.
func condBounded(pass *analysis.Pass, loop *ast.ForStmt, expr ast.Expr, p polarity) bool {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return condBounded(pass, loop, e.X, p)
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return condBounded(pass, loop, e.X, !p)
		}
		return false
	case *ast.BinaryExpr:
		if e.Op == token.LAND || e.Op == token.LOR {
			// && needs every operand bounded exactly when negated, ||
			// exactly when plain — the four cases of the De Morgan flip.
			if (e.Op == token.LAND) == (p == negatedContext) {
				return condBounded(pass, loop, e.X, p) && condBounded(pass, loop, e.Y, p)
			}
			return condBounded(pass, loop, e.X, p) || condBounded(pass, loop, e.Y, p)
		}
		return leafBounded(pass, loop, e, p)
	default:
		return false
	}
}

// leafBounded judges one comparison: the existing constant/len/cap-operand
// check for <, <=, >, >= (negation-invariant — a negated inequality is
// another inequality against the same stated bound); the monotone-counter
// widening for != 0; and the tuple-post-reversal widening for < and <=.
// The two widenings never apply under negation: !(x != 0) runs while x IS
// zero, so moving x toward zero keeps the loop alive, and the mirrored
// tuple argument inverts the same way.
func leafBounded(
	pass *analysis.Pass,
	loop *ast.ForStmt,
	binary *ast.BinaryExpr,
	p polarity,
) bool {
	comparison := binary.Op == token.LSS || binary.Op == token.LEQ ||
		binary.Op == token.GTR || binary.Op == token.GEQ
	boundOperand := isBoundOperand(pass, binary.X) || isBoundOperand(pass, binary.Y)
	if comparison && boundOperand {
		return true
	}
	if p == negatedContext {
		return false
	}
	if binary.Op == token.NEQ {
		return monotoneToZero(pass, loop, binary)
	}
	if binary.Op == token.LSS || binary.Op == token.LEQ {
		return tuplePostReversal(pass, loop, binary)
	}
	return false
}

// monotoneToZero reports whether binary is an "x != 0" comparison whose
// every assignment to x inside the loop (Post and Body, same frame,
// matched by object identity so a shadowed redeclaration is a different
// variable) provably moves x toward zero — a right shift, or a division
// by a constant greater than one — with at least one such assignment
// guaranteed to run every iteration: the Post clause, or a direct body
// statement no continue or goto before it can skip. A bare decrement does
// not qualify — a negative start would wrap instead of hitting zero — and
// any wrong-direction or unprovable assignment anywhere in the loop
// defeats the proof.
func monotoneToZero(pass *analysis.Pass, loop *ast.ForStmt, binary *ast.BinaryExpr) bool {
	ident, ok := neqZeroIdent(pass, binary)
	if !ok {
		return false
	}
	target, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	if !ok {
		return false
	}
	// The proof holds for integer counters only: float division never
	// moves NaN or Inf, and a signed arithmetic shift converges to -1
	// from a negative start, never zero — so shifts additionally require
	// an unsigned counter.
	basic, ok := target.Type().Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsInteger == 0 {
		return false
	}
	sign := signedCounter
	if basic.Info()&types.IsUnsigned != 0 {
		sign = unsignedCounter
	}
	assignments := loopAssignments(pass, target, loop)
	if len(assignments) == 0 {
		return false
	}
	for _, node := range assignments {
		if !zeroMovingAssignment(pass, node, sign) {
			return false
		}
	}
	return unconditionalAssignment(assignments, loop)
}

// unconditionalAssignment reports whether at least one collected
// assignment provably executes on every iteration: the Post clause itself
// (which Go runs even on continue), or a direct statement of the loop
// body with no continue or goto anywhere in the statements before it. An
// assignment nested under an if or switch is never guaranteed, no matter
// how likely its guard — a round that skips it leaves the counter where
// it was.
func unconditionalAssignment(assignments []ast.Node, loop *ast.ForStmt) bool {
	for _, node := range assignments {
		if loop.Post == node {
			return true
		}
	}
	for i, stmt := range loop.Body.List {
		for _, node := range assignments {
			if stmt == node {
				return !skipsBefore(loop.Body.List[:i])
			}
		}
	}
	return false
}

// skipsBefore reports whether any statement contains a continue or goto —
// a path that re-enters the condition without reaching what follows.
// A continue belonging to a nested loop is counted too: distinguishing
// targets needs label resolution, so the check stays conservative. A
// FuncLit cannot branch out of the enclosing loop, so it is skipped.
func skipsBefore(stmts []ast.Stmt) bool {
	for _, stmt := range stmts {
		skip := false
		ast.Inspect(stmt, func(n ast.Node) bool {
			if _, isFuncLit := n.(*ast.FuncLit); isFuncLit {
				return false
			}
			if branch, ok := n.(*ast.BranchStmt); ok {
				if branch.Tok == token.CONTINUE || branch.Tok == token.GOTO {
					skip = true
				}
			}
			return true
		})
		if skip {
			return true
		}
	}
	return false
}

// neqZeroIdent reports whether binary compares a plain identifier against
// a literal or resolved-constant zero, returning that identifier.
func neqZeroIdent(pass *analysis.Pass, binary *ast.BinaryExpr) (*ast.Ident, bool) {
	if binary.Op != token.NEQ {
		return nil, false
	}
	if ident, ok := binary.X.(*ast.Ident); ok && isZero(pass, binary.Y) {
		return ident, true
	}
	if ident, ok := binary.Y.(*ast.Ident); ok && isZero(pass, binary.X) {
		return ident, true
	}
	return nil, false
}

// isZero reports whether expr is the literal 0 or a resolved constant
// equal to zero.
func isZero(pass *analysis.Pass, expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.INT && e.Value == "0"
	case *ast.Ident:
		c, ok := pass.TypesInfo.Uses[e].(*types.Const)
		if !ok {
			return false
		}
		return constant.Sign(c.Val()) == 0
	}
	return false
}

// zeroMovingAssignment reports whether node — an assignment or IncDecStmt
// found by loopAssignments — provably moves its target toward zero: a
// right shift by a constant of at least one (x >>= k or x = x >> k,
// unsigned targets only) or a division by a constant greater than one
// (x /= c or x = x / c). A shift by a variable proves nothing (the amount
// can be zero), and an IncDecStmt (x++ or x--) never qualifies: a
// decrement risks wrapping past zero on a negative start.
func zeroMovingAssignment(pass *analysis.Pass, node ast.Node, sign counterSign) bool {
	assign, ok := node.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	if assign.Tok == token.SHR_ASSIGN {
		return sign == unsignedCounter && isConstAtLeast(pass, assign.Rhs[0], 1)
	}
	if assign.Tok == token.QUO_ASSIGN {
		return isConstAtLeast(pass, assign.Rhs[0], 2)
	}
	if assign.Tok == token.ASSIGN {
		return selfShiftOrDivide(pass, assign, sign)
	}
	return false
}

// selfShiftOrDivide reports whether assign's single RHS is "lhs >> k" or
// "lhs / c" with c a constant greater than one, the long forms of
// x >>= k / x /= c. Taking the whole assignment (rather than separate lhs
// and rhs expressions) keeps the two ast.Expr values out of the parameter
// list together.
func selfShiftOrDivide(pass *analysis.Pass, assign *ast.AssignStmt, sign counterSign) bool {
	lhsIdent, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return false
	}
	binary, ok := assign.Rhs[0].(*ast.BinaryExpr)
	if !ok {
		return false
	}
	rhsIdent, ok := binary.X.(*ast.Ident)
	if !ok || rhsIdent.Name != lhsIdent.Name {
		return false
	}
	if binary.Op == token.SHR {
		return sign == unsignedCounter && isConstAtLeast(pass, binary.Y, 1)
	}
	if binary.Op == token.QUO {
		return isConstAtLeast(pass, binary.Y, 2)
	}
	return false
}

// isConstAtLeast reports whether expr is a literal or resolved constant
// integer no smaller than min — the divisor floor is 2 (division by one
// never moves), the shift-amount floor is 1 (a shift by zero never moves).
func isConstAtLeast(pass *analysis.Pass, expr ast.Expr, min int64) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.INT {
			return false
		}
		n, err := strconv.ParseInt(e.Value, 0, 64)
		return err == nil && n >= min
	case *ast.Ident:
		c, ok := pass.TypesInfo.Uses[e].(*types.Const)
		if !ok {
			return false
		}
		return constant.Compare(c.Val(), token.GEQ, constant.MakeInt64(min))
	default:
		return false
	}
}

// tuplePostReversal reports whether binary (i < j or i <= j) is proven by
// a tuple Post that moves i up and j down by the same constant step,
// shrinking the gap every round, with neither identifier reassigned
// anywhere else in the loop body. Integer identifiers only: at large
// float magnitudes a step of +1 is absorbed (a+1 == a), so a float gap
// can stop shrinking.
func tuplePostReversal(pass *analysis.Pass, loop *ast.ForStmt, binary *ast.BinaryExpr) bool {
	loIdent, ok := binary.X.(*ast.Ident)
	if !ok || !integerTyped(pass, loIdent) {
		return false
	}
	hiIdent, ok := binary.Y.(*ast.Ident)
	if !ok || !integerTyped(pass, hiIdent) {
		return false
	}
	assign, ok := loop.Post.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != len(assign.Rhs) {
		return false
	}
	if !postShrinksGap(assign, gapNames{lo: loIdent.Name, hi: hiIdent.Name}) {
		return false
	}
	// The Cond and Post share the for-header scope, so name matching above
	// is object-safe; body reassignments need object identity to keep a
	// same-named shadow from spuriously defeating (or ever satisfying) the
	// no-reassignment requirement.
	loVar, ok := pass.TypesInfo.Uses[loIdent].(*types.Var)
	if !ok {
		return false
	}
	hiVar, ok := pass.TypesInfo.Uses[hiIdent].(*types.Var)
	if !ok {
		return false
	}
	if len(assignmentsTo(pass, loVar, loop.Body)) > 0 {
		return false
	}
	if len(assignmentsTo(pass, hiVar, loop.Body)) > 0 {
		return false
	}
	return true
}

// gapNames names the tuple-post pair a shrinking-gap proof tracks — the low
// identifier moving up and the high identifier moving down — keeping the
// two same-typed names out of postShrinksGap's parameter list.
type gapNames struct {
	lo, hi string
}

// postShrinksGap reports whether assign moves names.lo up by a constant and
// names.hi down by a constant, among its (possibly multi-target) pairs.
func postShrinksGap(assign *ast.AssignStmt, names gapNames) bool {
	loMoved, hiMoved := false, false
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		switch ident.Name {
		case names.lo:
			loMoved = stepDirection(names.lo, assign.Rhs[i]) > 0
		case names.hi:
			hiMoved = stepDirection(names.hi, assign.Rhs[i]) < 0
		}
	}
	return loMoved && hiMoved
}

// stepDirection reports +1 when rhs is "name + positive-const", -1 when
// rhs is "name - positive-const", and 0 for any other shape.
func stepDirection(name string, rhs ast.Expr) int {
	binary, ok := rhs.(*ast.BinaryExpr)
	if !ok {
		return 0
	}
	ident, ok := binary.X.(*ast.Ident)
	if !ok || ident.Name != name {
		return 0
	}
	lit, ok := binary.Y.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0
	}
	n, err := strconv.ParseInt(lit.Value, 0, 64)
	if err != nil || n <= 0 {
		return 0
	}
	if binary.Op == token.ADD {
		return 1
	}
	if binary.Op == token.SUB {
		return -1
	}
	return 0
}

// loopAssignments collects every assignment to target in loop's Post and
// Body, the span the monotone-counter proof scans.
func loopAssignments(pass *analysis.Pass, target *types.Var, loop *ast.ForStmt) []ast.Node {
	var found []ast.Node
	if loop.Post != nil {
		found = append(found, assignmentsTo(pass, target, loop.Post)...)
	}
	found = append(found, assignmentsTo(pass, target, loop.Body)...)
	return found
}

// assignmentsTo collects every AssignStmt, IncDecStmt, or range clause
// targeting target within node, without descending into a FuncLit: a
// closure is a frame boundary, so a mutation inside it counts neither for
// nor against the direction proof, matching deferdistance's and
// declusedistance's treatment of the same shape. Matching is by object
// identity through TypesInfo.Uses — a := redeclaration defines a new
// object (a shadow), so its assignments count neither for nor against the
// outer variable's proof. A range clause counts because "for x = range s"
// reassigns x — an inner range can reset the counter after every
// qualifying move.
func assignmentsTo(pass *analysis.Pass, target *types.Var, node ast.Node) []ast.Node {
	uses := func(expr ast.Expr) bool {
		ident, ok := expr.(*ast.Ident)
		return ok && pass.TypesInfo.Uses[ident] == target
	}
	var found []ast.Node
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if _, isFuncLit := n.(*ast.FuncLit); isFuncLit {
			return false
		}
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				if uses(lhs) {
					found = append(found, stmt)
				}
			}
		case *ast.IncDecStmt:
			if uses(stmt.X) {
				found = append(found, stmt)
			}
		case *ast.RangeStmt:
			if uses(stmt.Key) || uses(stmt.Value) {
				found = append(found, stmt)
			}
		}
		return true
	})
	return found
}

// integerTyped reports whether ident resolves to an integer-typed value.
func integerTyped(pass *analysis.Pass, ident *ast.Ident) bool {
	identType := pass.TypesInfo.TypeOf(ident)
	if identType == nil {
		return false
	}
	basic, ok := identType.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsInteger != 0
}

// cursorShape reports whether loop's Cond is a method call on a plain
// identifier — the it.Valid() / rows.Next() store-cursor pattern the
// //tiger:batched waiver recognizes. A package-qualified call such as
// strings.Contains is not a method on a cursor value, so it stays
// outside the waiver's shape gate.
func cursorShape(pass *analysis.Pass, loop *ast.ForStmt) bool {
	call, ok := loop.Cond.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, isPkg := pass.TypesInfo.Uses[ident].(*types.PkgName)
	return !isPkg
}

// postCounter returns the identifier name a Post clause advances by
// IncDecStmt or by += / -=, or "" when Post proves no counter.
func postCounter(post ast.Stmt) string {
	switch stmt := post.(type) {
	case *ast.IncDecStmt:
		if ident, ok := stmt.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.AssignStmt:
		if stmt.Tok != token.ADD_ASSIGN && stmt.Tok != token.SUB_ASSIGN {
			return ""
		}
		if len(stmt.Lhs) == 1 {
			if ident, ok := stmt.Lhs[0].(*ast.Ident); ok {
				return ident.Name
			}
		}
	}
	return ""
}

// identifierAppears reports whether name is referenced anywhere inside expr.
func identifierAppears(expr ast.Expr, name string) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && ident.Name == name {
			found = true
		}
		return true
	})
	return found
}

// isBoundOperand reports whether expr is a literal, a resolved constant, or
// a len()/cap() call — the shapes TS-S02 accepts as a stated bound.
func isBoundOperand(pass *analysis.Pass, expr ast.Expr) bool {
	switch operand := expr.(type) {
	case *ast.ParenExpr:
		return isBoundOperand(pass, operand.X)
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		_, isConst := pass.TypesInfo.Uses[operand].(*types.Const)
		return isConst
	case *ast.SelectorExpr:
		_, isConst := pass.TypesInfo.Uses[operand.Sel].(*types.Const)
		return isConst
	case *ast.CallExpr:
		return isLenOrCap(pass, operand)
	default:
		return false
	}
}

// isLenOrCap reports whether call is a call to the builtin len or cap.
func isLenOrCap(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return false
	}
	return builtin.Name() == "len" || builtin.Name() == "cap"
}
