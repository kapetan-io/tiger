// Package boundedloop enforces TS-S02 and TS-S03: every loop has an upper
// bound, and an unbounded event loop selects on ctx.Done().
//
// A RangeStmt over a slice, array, map, string, integer, or range-over-func
// iterator is bounded by its own syntax. A RangeStmt over a channel is not:
// termination depends on another goroutine closing it, so it is a TS-S02
// finding. A ForStmt with no Cond ("for {}") is either the TS-S03 event-loop
// shape — a direct SelectStmt with a case receiving from ctx.Done() — or a
// TS-S02 finding. A ForStmt with a Cond is bounded when its Post advances a
// variable that appears in Cond (a counter), or when Cond compares against a
// constant, a len(), or a cap(); any other Cond is a TS-S02 finding.
package boundedloop

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
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
		"ctx.Done() — add a case <-ctx.Done(): return so the loop has a termination path"
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
				checkFor(pass, loop)
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
// bounded three-clause loop, or a TS-S02 finding.
func checkFor(pass *analysis.Pass, loop *ast.ForStmt) {
	if loop.Cond == nil {
		checkEventLoop(pass, loop)
		return
	}
	if boundedCond(pass, loop) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      loop.Cond.Pos(),
		Category: "TS-S02",
		Message:  msgUnbounded,
	})
}

// checkEventLoop handles a for{} with no condition: the TS-S03 shape (a
// direct select with a ctx.Done() case), a select missing that case
// (TS-S03), or no select at all (TS-S02). A select with a default clause
// is non-blocking — a drain loop, not an event loop waiting on the outside
// world — so it needs no shutdown case, matching TS-C05's treatment of the
// same shape.
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
	if !selectHasDoneCase(sel) {
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

// selectHasDoneCase reports whether the select has a comm case that
// receives from a call to a method named Done, the ctx.Done() shape.
func selectHasDoneCase(sel *ast.SelectStmt) bool {
	for _, clause := range sel.Body.List {
		comm, ok := clause.(*ast.CommClause)
		if ok && receivesFromDoneCall(comm.Comm) {
			return true
		}
	}
	return false
}

// receivesFromDoneCall reports whether comm receives from a call whose
// callee is a method named Done, covering both "<-ctx.Done()" and
// "v, ok := <-ctx.Done()" forms. A nil comm is a select's default case.
func receivesFromDoneCall(comm ast.Stmt) bool {
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
	call, ok := unary.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Done"
}

// boundedCond reports whether loop's Cond/Post pair proves a bound: a
// counter advanced in Post that appears in Cond, or a Cond comparison
// against a constant, len(), or cap().
func boundedCond(pass *analysis.Pass, loop *ast.ForStmt) bool {
	if counter := postCounter(loop.Post); counter != "" && identifierAppears(loop.Cond, counter) {
		return true
	}
	binary, ok := loop.Cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	comparison := binary.Op == token.LSS || binary.Op == token.LEQ ||
		binary.Op == token.GTR || binary.Op == token.GEQ
	if !comparison {
		return false
	}
	return isBoundOperand(pass, binary.X) || isBoundOperand(pass, binary.Y)
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
