package boundedloop

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// checkExits reports TS-S23 for every return and break-out of loop that sits
// outside a stop case. The loop's only way out is the stop case, so whatever
// runs after the stop, such as closing done and answering waiters, runs every
// time. The stop case is a case on a live ctx.Done() or a channel the package
// closes. A case on a channel the package only sends on counts only when it
// ends in a return: that channel may carry requests as easily as a shutdown
// handshake, and the case that returns unconditionally is the handshake.
// A return inside a function literal leaves that function, not the loop, and
// an unlabeled break inside a nested select, switch or loop leaves that
// statement, so neither counts. A labeled break or a goto counts unless the
// label is declared inside the loop.
func (c *checker) checkExits(loop *ast.ForStmt) {
	inner := labelsWithin(loop.Body)
	own := c.labelOf(loop)
	var ancestors []ast.Node
	ast.Inspect(loop.Body, func(node ast.Node) bool {
		if node == nil {
			ancestors = ancestors[:len(ancestors)-1]
			return false
		}
		switch stmt := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CommClause:
			if c.stopClause(stmt) {
				return false
			}
		case *ast.ReturnStmt:
			c.reportExit(stmt.Pos(), "return")
		case *ast.BranchStmt:
			if exitsLoop(stmt, inner, own, ancestors) {
				c.reportExit(stmt.Pos(), strings.ToLower(stmt.Tok.String()))
			}
		}
		ancestors = append(ancestors, node)
		return true
	})
}

// reportExit reports one way out of an event loop other than its stop case.
func (c *checker) reportExit(pos token.Pos, kind string) {
	c.pass.Report(analysis.Diagnostic{
		Pos: pos, Category: "TS-S23", Message: fmt.Sprintf(msgExitsOutside, kind),
	})
}

// exitsLoop reports whether a break or goto statement ends the event loop: a
// jump to a label outside the loop, or an unlabeled break with no select,
// switch or loop between it and the loop.
func exitsLoop(
	stmt *ast.BranchStmt, inner map[string]bool, own string, ancestors []ast.Node,
) bool {
	if stmt.Tok != token.BREAK && stmt.Tok != token.GOTO {
		return false
	}
	if stmt.Label != nil {
		if stmt.Tok == token.GOTO && stmt.Label.Name == own {
			return false
		}
		return !inner[stmt.Label.Name]
	}
	for _, node := range ancestors {
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		}
	}
	return true
}

// labelsWithin returns the names of the labels declared inside body.
func labelsWithin(body *ast.BlockStmt) map[string]bool {
	names := map[string]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		if labeled, ok := node.(*ast.LabeledStmt); ok {
			names[labeled.Label.Name] = true
		}
		return true
	})
	return names
}

// stopClause reports whether clause is a stop case of an event loop, whose
// returns TS-S23 allows.
func (c *checker) stopClause(clause *ast.CommClause) bool {
	if clause.Comm == nil || !c.stops.Case(clause.Comm) {
		return false
	}
	return c.stops.Closes(clause.Comm) || endsInReturn(clause.Body)
}

// endsInReturn reports whether a case body's last statement is a return.
func endsInReturn(body []ast.Stmt) bool {
	if len(body) == 0 {
		return false
	}
	_, ok := body[len(body)-1].(*ast.ReturnStmt)
	return ok
}

// labelOf returns the label on loop, or "" if it has none. A goto to the
// loop's own label restarts the loop and does not leave it.
func (c *checker) labelOf(loop *ast.ForStmt) string {
	name := ""
	for _, file := range c.pass.Files {
		if file.Pos() > loop.Pos() || loop.End() > file.End() {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if labeled, ok := node.(*ast.LabeledStmt); ok && labeled.Stmt == ast.Stmt(loop) {
				name = labeled.Label.Name
			}
			return name == ""
		})
	}
	return name
}

// leavesLoop reports whether a case body returns from the function or breaks
// out of the loop. A return inside a function literal, or a break that leaves
// an inner statement, does not count.
func (c *checker) leavesLoop(loop *ast.ForStmt, clause *ast.CommClause) bool {
	inner := labelsWithin(loop.Body)
	own := c.labelOf(loop)
	ancestors := []ast.Node{&ast.SelectStmt{}}
	leaves := false
	for _, stmt := range clause.Body {
		ast.Inspect(stmt, func(node ast.Node) bool {
			if node == nil {
				ancestors = ancestors[:len(ancestors)-1]
				return false
			}
			switch found := node.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ReturnStmt:
				leaves = true
			case *ast.BranchStmt:
				leaves = leaves || exitsLoop(found, inner, own, ancestors)
			}
			ancestors = append(ancestors, node)
			return !leaves
		})
	}
	return leaves
}
