// Package selectctx enforces TS-C05: every blocking receive or send has a
// stop case.
//
// A select with no default clause blocks, so it must carry a stop case: a
// receive from ctx.Done() on a context that does not trace to Background or
// TODO within the function, or a receive or send on a channel the package
// itself closes or sends on (see internal/analyzers/internal/stopcase). A
// select with a default clause is non-blocking and needs none. A send, or a
// receive that sits outside any select's own comm clause, blocks
// unconditionally, so it must be wrapped in a select — unless it is a receive
// that is itself the wait for shutdown, from a live ctx.Done() or a channel
// the package closes. A channel's name never exempts a bare operation, and
// neither does its element type: outside a select, a struct{} channel is just
// as often a completion wait with no cancellation, the very bug this rule
// catches.
//
// Known miss: a bare receive from a channel the package closes is accepted
// even when it is a completion wait rather than a shutdown wait.
package selectctx

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/stopcase"
)

// Analyzer enforces TS-C05: every blocking receive or send has a stop case.
var Analyzer = &analysis.Analyzer{
	Name: "selectctx",
	Doc:  "TS-C05: every blocking receive or send has a stop case.",
	Run:  run,
}

const (
	msgNoShutdown = "TS-C05: this select blocks with no case that ends the wait on shutdown — " +
		"add case <-ctx.Done(): return ctx.Err(), or a case on a channel this package closes"
	msgUnwrapped = "TS-C05: this channel operation blocks outside a select, so shutdown can't " +
		"interrupt it — wrap it in a select that also has case <-ctx.Done(): return ctx.Err()"
)

func run(pass *analysis.Pass) (any, error) {
	stops := stopcase.New(pass)
	for _, file := range pass.Files {
		protected := commHeadlines(file)
		ast.Inspect(file, func(node ast.Node) bool {
			switch stmt := node.(type) {
			case *ast.SelectStmt:
				checkSelect(pass, stops, stmt)
			case *ast.SendStmt:
				if !protected[stmt] {
					pass.Report(analysis.Diagnostic{
						Pos: stmt.Pos(), Category: "TS-C05", Message: msgUnwrapped,
					})
				}
			case *ast.UnaryExpr:
				// A bare receive from a live ctx.Done() or a channel the
				// package closes is itself the shutdown wait; asking to
				// wrap it in a select would restate the code that is
				// already there.
				if stmt.Op == token.ARROW && !protected[stmt] && !stops.BareReceive(stmt) {
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
// carries no stop case.
func checkSelect(pass *analysis.Pass, stops *stopcase.Finder, sel *ast.SelectStmt) {
	if hasDefault(sel) || hasStopCase(stops, sel) {
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

// hasStopCase reports whether sel carries a case that ends the wait.
func hasStopCase(stops *stopcase.Finder, sel *ast.SelectStmt) bool {
	for _, clause := range sel.Body.List {
		if comm, ok := clause.(*ast.CommClause); ok && stops.Case(comm.Comm) {
			return true
		}
	}
	return false
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
			if arrow := stopcase.Receive(comm.Comm); arrow != nil {
				protected[arrow] = true
			}
		}
		return true
	})
	return protected
}
