package boundedloop

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/counter"
)

const msgCapSilent = "TS-S02: this counter cap ends the loop without saying so, so a truncated " +
	"result passes as complete — declare the counter before the loop and return an error or " +
	"assert when it reaches the limit"

// checkCap judges a loop a sound counter bounds. A counter whose limit is
// all that stops the loop is fine. A counter that stands in for a proof,
// because the loop has another way out, is a safety cap and must report
// when it is hit, unless it is a page size over a stream from outside the
// module.
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

// capTerms returns the sound terms that act as safety caps: a limit that is
// not a len or cap, with another way out of the loop that is not a page
// size over an outside stream.
func (c *checker) capTerms(loop *ast.ForStmt, terms []counter.Term) []counter.Term {
	var caps []counter.Term
	broken := breaksOut(c.pass.TypesInfo, loop)
	for _, term := range terms {
		if lengthLimit(c.pass.TypesInfo, term.Limit) {
			continue
		}
		others := otherExits(loop.Cond, term)
		if len(others) == 0 && !broken {
			continue
		}
		if !broken && c.pageSize(term, others) {
			continue
		}
		caps = append(caps, term)
	}
	return caps
}

// lengthLimit reports whether limit is a len or cap call, a bound the data
// itself sets.
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

// lengthBuiltins are the builtins that read a length the data sets.
var lengthBuiltins = map[string]bool{"len": true, "cap": true}

// reachedComparisons are the comparisons a cap check makes between the
// counter and the limit.
var reachedComparisons = map[token.Token]bool{
	token.EQL: true, token.GEQ: true, token.GTR: true, token.LEQ: true, token.LSS: true,
}

// otherExits returns the top-level terms of cond besides term's own
// comparison.
func otherExits(cond ast.Expr, term counter.Term) []ast.Expr {
	var others []ast.Expr
	for _, conjunct := range counter.Conjuncts(cond) {
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

// pageSize reports whether term's limit is a page size: not a constant, and
// every other exit drains a stream whose type comes from outside the module.
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

// drainsOutsideStream reports whether expr is a boolean method call on a
// type declared outside the module, such as rows.Next() or scanner.Scan().
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
	named := receiverNamed(selection.Recv())
	return named != nil && c.outsideModule(named.Obj().Pkg())
}

// receiverNamed returns the named type behind a receiver or a pointer to
// one.
func receiverNamed(t types.Type) *types.Named {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, _ := t.(*types.Named)
	return named
}

// outsideModule reports whether pkg is declared outside the module under
// analysis. Without module information, any package but the one under
// analysis counts as outside.
func (c *checker) outsideModule(pkg *types.Package) bool {
	if pkg == nil || pkg == c.pass.Pkg {
		return false
	}
	if c.pass.Module == nil || c.pass.Module.Path == "" {
		return true
	}
	path := c.pass.Module.Path
	return pkg.Path() != path && !strings.HasPrefix(pkg.Path(), path+"/")
}

// breaksOut reports whether the loop body has a break that ends this loop:
// an unlabeled break outside any nested loop, switch or select, or a labeled
// break to a label declared outside the body.
func breaksOut(info *types.Info, loop *ast.ForStmt) bool {
	found := false
	ast.Inspect(loop.Body, func(n ast.Node) bool {
		branch, ok := n.(*ast.BranchStmt)
		if ok && branch.Tok == token.BREAK && endsThisLoop(info, loop, branch) {
			found = true
		}
		return !found
	})
	return found
}

// endsThisLoop reports whether branch, found somewhere in loop's body,
// leaves loop itself.
func endsThisLoop(info *types.Info, loop *ast.ForStmt, branch *ast.BranchStmt) bool {
	if branch.Label != nil {
		label := info.ObjectOf(branch.Label)
		return label != nil && (label.Pos() < loop.Pos() || label.Pos() >= loop.End())
	}
	return !insideBreakTarget(loop.Body, branch)
}

// insideBreakTarget reports whether an unlabeled break sits inside a loop,
// switch or select nested in body, where it ends that statement instead.
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

// silentWhenHit reports whether nothing after the loop reports the cap being
// reached. Two shapes report. One is an if statement that compares the
// counter with the limit and asserts, panics, or returns an error. The other
// is an assert or panic statement on its own, which states what the loop was
// waiting for; tiger can't tell what the assertion is about, so it trusts the
// author that it covers the cap.
func (c *checker) silentWhenHit(loop *ast.ForStmt, caps []counter.Term) bool {
	for _, stmt := range c.following[loop] {
		switch next := stmt.(type) {
		case *ast.ExprStmt:
			if call, ok := next.X.(*ast.CallExpr); ok && c.assertsOrPanics(call) {
				return false
			}
		case *ast.IfStmt:
			for _, term := range caps {
				if c.comparesAtLimit(next.Cond, term) && c.fails(next.Body) {
					return false
				}
			}
		}
	}
	return true
}

// comparesAtLimit reports whether cond holds a comparison of term's counter
// with its limit that is true once the counter has reached the limit.
func (c *checker) comparesAtLimit(cond ast.Expr, term counter.Term) bool {
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		binary, ok := n.(*ast.BinaryExpr)
		if ok && reachedComparisons[binary.Op] && c.pairsWith(binary, term) {
			found = true
		}
		return !found
	})
	return found
}

// pairsWith reports whether one side of binary names term's counter and the
// other reads like its limit.
func (c *checker) pairsWith(binary *ast.BinaryExpr, term counter.Term) bool {
	if ident, ok := ast.Unparen(binary.X).(*ast.Ident); ok {
		if c.pass.TypesInfo.ObjectOf(ident) == term.Counter && c.readsLikeLimit(binary.Y, term) {
			return true
		}
	}
	ident, ok := ast.Unparen(binary.Y).(*ast.Ident)
	return ok && c.pass.TypesInfo.ObjectOf(ident) == term.Counter && c.readsLikeLimit(binary.X, term)
}

// readsLikeLimit reports whether expr is term's limit, written the same way
// or folded to the same constant.
func (c *checker) readsLikeLimit(expr ast.Expr, term counter.Term) bool {
	if types.ExprString(ast.Unparen(expr)) == types.ExprString(ast.Unparen(term.Limit)) {
		return true
	}
	known := c.pass.TypesInfo.Types[expr].Value
	want := c.pass.TypesInfo.Types[term.Limit].Value
	return known != nil && want != nil && known.ExactString() == want.ExactString()
}

// fails reports whether body asserts, panics, or returns a non-nil error.
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

// returnsError reports whether stmt returns a value that is not nil in an
// error position.
func (c *checker) returnsError(stmt *ast.ReturnStmt) bool {
	for _, result := range stmt.Results {
		resultType := c.pass.TypesInfo.TypeOf(result)
		if resultType == nil || isNil(c.pass.TypesInfo, result) {
			continue
		}
		if types.Implements(resultType, errorInterface()) {
			return true
		}
	}
	return false
}

// errorInterface returns the predeclared error interface.
func errorInterface() *types.Interface {
	return types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
}

// assertsOrPanics reports whether call is the panic builtin or a function
// from a package named assert.
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
		return ok && pkg.Imported().Name() == "assert"
	}
	return false
}
