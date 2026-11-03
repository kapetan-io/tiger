package boundedloop

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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

func (c *checker) reportExit(pos token.Pos, kind string) {
	c.pass.Report(analysis.Diagnostic{
		Pos: pos, Category: "TS-S23", Message: fmt.Sprintf(msgExitsOutside, kind),
	})
}

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

func (c *checker) stopClause(clause *ast.CommClause) bool {
	if clause.Comm == nil || !c.stops.Case(clause.Comm) {
		return false
	}
	return c.stops.Closes(clause.Comm) || endsInReturn(clause.Body)
}

func endsInReturn(body []ast.Stmt) bool {
	if len(body) == 0 {
		return false
	}
	_, ok := body[len(body)-1].(*ast.ReturnStmt)
	return ok
}

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
