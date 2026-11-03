package boundedloop

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/assertpkg"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/condition"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/counter"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
)

const msgCapSilent = "TS-S02: this counter cap ends the loop without saying so, so a truncated " +
	"result passes as complete — declare the counter before the loop and return an error or " +
	"assert when it reaches the limit"

func (c *checker) checkCap(loop *ast.ForStmt, terms []counter.Term) {
	caps := c.capTerms(loop, terms)
	if len(caps) == 0 || !c.silentWhenHit(loop, caps) {
		return
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:      loop.Cond.Pos(),
		Category: "TS-S02",
		Message:  msgCapSilent,
	})
}

func (c *checker) capTerms(loop *ast.ForStmt, terms []counter.Term) []counter.Term {
	var caps []counter.Term
	breaks := loopBreaks(c.pass.TypesInfo, loop)
	broken := len(breaks) > 0
	for _, term := range terms {
		if lengthLimit(c.pass.TypesInfo, term.Limit) {
			continue
		}
		others := otherExits(loop.Cond, term)
		if len(others) == 0 && !broken {
			continue
		}
		if c.pageSize(term, others) {
			if c.breaksDrain(loop, breaks) {
				continue
			}
		}
		caps = append(caps, term)
	}
	return caps
}

func lengthLimit(info *types.Info, limit ast.Expr) bool {
	call, ok := ast.Unparen(limit).(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := info.Uses[ident].(*types.Builtin)
	return ok && lengthBuiltins[builtin.Name()]
}

var lengthBuiltins = map[string]bool{"len": true, "cap": true}

var reachedComparisons = map[bool]map[bool]map[token.Token]bool{
	false: {
		false: {token.EQL: true, token.GEQ: true, token.LEQ: true},
		true:  {token.GEQ: true, token.GTR: true},
	},
	true: {
		false: {token.EQL: true, token.LEQ: true, token.GEQ: true},
		true:  {token.LEQ: true, token.LSS: true},
	},
}

var steppedComparisons = map[bool]map[bool]map[token.Token]bool{
	false: {
		false: {token.GEQ: true},
		true:  {token.GEQ: true, token.GTR: true},
	},
	true: {
		false: {token.LEQ: true},
		true:  {token.LEQ: true, token.LSS: true},
	},
}

func reachedBy(term counter.Term, op token.Token) bool {
	if term.Size > 1 {
		return steppedComparisons[term.Down][term.Inclusive][op]
	}
	return reachedComparisons[term.Down][term.Inclusive][op]
}

func otherExits(cond ast.Expr, term counter.Term) []ast.Expr {
	var others []ast.Expr
	for _, conjunct := range condition.Conjuncts(cond) {
		binary, ok := conjunct.(*ast.BinaryExpr)
		if !ok {
			others = append(others, conjunct)
			continue
		}
		if binary.X != term.Limit && binary.Y != term.Limit {
			others = append(others, conjunct)
		}
	}
	return others
}

func (c *checker) pageSize(term counter.Term, others []ast.Expr) bool {
	if c.pass.TypesInfo.Types[term.Limit].Value != nil {
		return false
	}
	for _, other := range others {
		if !c.drainsOutsideStream(other) {
			return false
		}
	}
	return true
}

func (c *checker) drainsOutsideStream(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	selection := c.pass.TypesInfo.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodVal {
		return false
	}
	if !streamMethods[selector.Sel.Name] {
		return false
	}
	named := mutation.DerefNamed(selection.Recv())
	return named != nil && c.scope.External(named.Obj().Pkg())
}

var streamMethods = map[string]bool{"Next": true, "Scan": true}

func loopBreaks(info *types.Info, loop *ast.ForStmt) []*ast.BranchStmt {
	var found []*ast.BranchStmt
	ast.Inspect(loop.Body, func(n ast.Node) bool {
		branch, ok := n.(*ast.BranchStmt)
		if ok && branch.Tok == token.BREAK && endsThisLoop(info, loop, branch) {
			found = append(found, branch)
		}
		return true
	})
	return found
}

func (c *checker) breaksDrain(loop *ast.ForStmt, breaks []*ast.BranchStmt) bool {
	drains := 0
	ast.Inspect(loop.Body, func(n ast.Node) bool {
		guard, ok := n.(*ast.IfStmt)
		if ok && guard.Init == nil && c.drainBreak(guard, breaks) {
			drains++
		}
		return true
	})
	return drains == len(breaks)
}

func (c *checker) drainBreak(guard *ast.IfStmt, breaks []*ast.BranchStmt) bool {
	cond := ast.Unparen(guard.Cond)
	var exit *ast.BlockStmt
	negated, isNot := cond.(*ast.UnaryExpr)
	switch {
	case isNot && negated.Op == token.NOT && guard.Else == nil:
		cond, exit = ast.Unparen(negated.X), guard.Body
	case !isNot:
		if other, isBlock := guard.Else.(*ast.BlockStmt); isBlock {
			exit = other
		}
	}
	if exit == nil || len(exit.List) != 1 {
		return false
	}
	branch, ok := exit.List[0].(*ast.BranchStmt)
	return ok && slices.Contains(breaks, branch) && c.drainsOutsideStream(cond)
}

func endsThisLoop(info *types.Info, loop *ast.ForStmt, branch *ast.BranchStmt) bool {
	if branch.Label != nil {
		label := info.ObjectOf(branch.Label)
		return label != nil && (label.Pos() < loop.Pos() || label.Pos() >= loop.End())
	}
	return !insideBreakTarget(loop.Body, branch)
}

func insideBreakTarget(body *ast.BlockStmt, branch *ast.BranchStmt) bool {
	inside := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if n.Pos() <= branch.Pos() && branch.End() <= n.End() {
				inside = true
			}
		}
		return !inside
	})
	return inside
}

func (c *checker) silentWhenHit(loop *ast.ForStmt, caps []counter.Term) bool {
	for _, stmt := range c.following[loop] {
		switch next := stmt.(type) {
		case *ast.ExprStmt:
			if call, ok := next.X.(*ast.CallExpr); ok && c.assertsOrPanics(call) {
				return false
			}
		case *ast.IfStmt:
			if !c.fails(next.Body) {
				continue
			}
			if c.readsLoopResult(next.Cond, loop) {
				return false
			}
			for _, term := range caps {
				if c.comparesAtLimit(next.Cond, term) {
					return false
				}
			}
		}
	}
	return true
}

func (c *checker) readsLoopResult(cond ast.Expr, loop *ast.ForStmt) bool {
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return !found
		}
		if v, isVar := c.pass.TypesInfo.ObjectOf(ident).(*types.Var); isVar {
			found = len(c.scope.Writes(loop.Body, v)) > 0
		}
		return !found
	})
	return found
}

func (c *checker) comparesAtLimit(cond ast.Expr, term counter.Term) bool {
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		binary, ok := n.(*ast.BinaryExpr)
		if !ok {
			return !found
		}
		if op, pairs := c.counterOnLeft(binary, term); pairs && reachedBy(term, op) {
			found = true
		}
		return !found
	})
	return found
}

func (c *checker) counterOnLeft(binary *ast.BinaryExpr, term counter.Term) (token.Token, bool) {
	if ident, ok := ast.Unparen(binary.X).(*ast.Ident); ok {
		if c.pass.TypesInfo.ObjectOf(ident) == term.Counter && c.readsLikeLimit(binary.Y, term) {
			return binary.Op, true
		}
	}
	ident, ok := ast.Unparen(binary.Y).(*ast.Ident)
	if !ok || c.pass.TypesInfo.ObjectOf(ident) != term.Counter {
		return token.ILLEGAL, false
	}
	if !c.readsLikeLimit(binary.X, term) {
		return token.ILLEGAL, false
	}
	if binary.Op == token.EQL {
		return token.EQL, true
	}
	return condition.Mirror(binary.Op)
}

func (c *checker) readsLikeLimit(expr ast.Expr, term counter.Term) bool {
	if types.ExprString(ast.Unparen(expr)) == types.ExprString(ast.Unparen(term.Limit)) {
		return true
	}
	known := c.pass.TypesInfo.Types[expr].Value
	want := c.pass.TypesInfo.Types[term.Limit].Value
	return known != nil && want != nil && known.ExactString() == want.ExactString()
}

func (c *checker) fails(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.ReturnStmt:
			if c.returnsError(stmt) {
				found = true
			}
		case *ast.CallExpr:
			if c.assertsOrPanics(stmt) {
				found = true
			}
		}
		return !found
	})
	return found
}

func (c *checker) returnsError(stmt *ast.ReturnStmt) bool {
	for _, result := range stmt.Results {
		resultType := c.pass.TypesInfo.TypeOf(result)
		if resultType == nil || condition.IsNil(c.pass.TypesInfo, result) {
			continue
		}
		if types.Implements(resultType, errorInterface()) {
			return true
		}
	}
	return false
}

func errorInterface() *types.Interface {
	iface, ok := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	assert.Ok(ok, "errorInterface: the universe error type is not an interface")
	return iface
}

func (c *checker) assertsOrPanics(call *ast.CallExpr) bool {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		builtin, ok := c.pass.TypesInfo.Uses[fun].(*types.Builtin)
		return ok && builtin.Name() == "panic"
	case *ast.SelectorExpr:
		pkgIdent, ok := fun.X.(*ast.Ident)
		if !ok {
			return false
		}
		pkg, ok := c.pass.TypesInfo.Uses[pkgIdent].(*types.PkgName)
		return ok && assertpkg.Is(pkg.Imported().Path())
	}
	return false
}
