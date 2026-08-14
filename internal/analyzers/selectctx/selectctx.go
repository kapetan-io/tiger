// Package selectctx enforces TS-C05: every blocking receive or send has a
// ctx.Done() case.
//
// A select with no default clause blocks, so it must carry a comm case
// receiving from a call to a method named Done — the ctx.Done() shape — or
// shutdown has no path out. A select with a default clause is non-blocking
// and needs none. A send or a bare receive that sits outside any select's
// own comm clause blocks unconditionally and has no shutdown path at all,
// so it must be wrapped in a select instead.
package selectctx

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-C05: every blocking receive or send has a
// ctx.Done() case.
var Analyzer = &analysis.Analyzer{
	Name: "selectctx",
	Doc:  "TS-C05: every blocking receive or send has a ctx.Done() case.",
	Run:  run,
}

const (
	msgNoShutdown = "TS-C05: blocking select has no shutdown case — add " +
		"`case <-ctx.Done(): return ctx.Err()` so shutdown does not hang"
	msgUnwrapped = "TS-C05: blocking channel operation outside a select — wrap it in a select " +
		"with a `case <-ctx.Done(): return ctx.Err()` case"
)

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		protected := commHeadlines(file)
		ast.Inspect(file, func(node ast.Node) bool {
			switch stmt := node.(type) {
			case *ast.SelectStmt:
				checkSelect(pass, stmt)
			case *ast.SendStmt:
				if !protected[stmt] {
					pass.Report(analysis.Diagnostic{
						Pos: stmt.Pos(), Category: "TS-C05", Message: msgUnwrapped,
					})
				}
			case *ast.UnaryExpr:
				if stmt.Op == token.ARROW && !protected[stmt] {
					pass.Report(analysis.Diagnostic{
						Pos: stmt.Pos(), Category: "TS-C05", Message: msgUnwrapped,
					})
				}
			}
			return true
		})
	}
	return nil, nil
}

// checkSelect fires TS-C05 for a blocking select (no default clause) that
// carries no case receiving from a call to a method named Done.
func checkSelect(pass *analysis.Pass, sel *ast.SelectStmt) {
	if hasDefault(sel) || hasDoneCase(sel) {
		return
	}
	pass.Report(analysis.Diagnostic{Pos: sel.Pos(), Category: "TS-C05", Message: msgNoShutdown})
}

// hasDefault reports whether sel carries a default clause, which makes it
// non-blocking.
func hasDefault(sel *ast.SelectStmt) bool {
	for _, clause := range sel.Body.List {
		if comm, ok := clause.(*ast.CommClause); ok && comm.Comm == nil {
			return true
		}
	}
	return false
}

// hasDoneCase reports whether sel carries a comm case receiving from a
// call to a method named Done, the ctx.Done() shape.
func hasDoneCase(sel *ast.SelectStmt) bool {
	for _, clause := range sel.Body.List {
		if comm, ok := clause.(*ast.CommClause); ok && receivesFromDone(comm.Comm) {
			return true
		}
	}
	return false
}

// receivesFromDone reports whether comm — a select case's headline
// statement — receives from a call whose callee is a method named Done,
// covering both "case <-ctx.Done():" and "case v := <-ctx.Done():".
func receivesFromDone(comm ast.Stmt) bool {
	arrow := commArrow(comm)
	if arrow == nil {
		return false
	}
	call, ok := arrow.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Done"
}

// commHeadlines collects every select case's headline receive expression
// and headline send statement across file, so the generic walk can skip
// flagging a select's own comm operation as a blocking op outside a
// select.
func commHeadlines(file *ast.File) map[ast.Node]bool {
	protected := map[ast.Node]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectStmt)
		if !ok {
			return true
		}
		for _, clause := range sel.Body.List {
			comm, ok := clause.(*ast.CommClause)
			if !ok || comm.Comm == nil {
				continue
			}
			if send, ok := comm.Comm.(*ast.SendStmt); ok {
				protected[send] = true
				continue
			}
			if arrow := commArrow(comm.Comm); arrow != nil {
				protected[arrow] = true
			}
		}
		return true
	})
	return protected
}

// commArrow extracts the receive expression from a comm clause's headline
// statement, covering "<-ch" and "v := <-ch" / "v, ok := <-ch" forms.
func commArrow(comm ast.Stmt) *ast.UnaryExpr {
	var expr ast.Expr
	switch stmt := comm.(type) {
	case *ast.ExprStmt:
		expr = stmt.X
	case *ast.AssignStmt:
		if len(stmt.Rhs) == 1 {
			expr = stmt.Rhs[0]
		}
	}
	arrow, ok := expr.(*ast.UnaryExpr)
	if !ok || arrow.Op != token.ARROW {
		return nil
	}
	return arrow
}
