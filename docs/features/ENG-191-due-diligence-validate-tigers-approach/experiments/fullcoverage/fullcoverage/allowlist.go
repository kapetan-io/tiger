package fullcoverage

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// The allowlist is tiger's maporder allowlist with two changes. First, when
// carried is non-nil, no read may see state an earlier visit wrote: such a
// read lets visit order pick which entries are written or what is written.
// The accumulator forms (x += e, x = max(x, e), if e < x { x = e }) read
// their own target by design; only e is checked for them. A nil carried
// skips the check, which classifyMessage uses to name the cause. Second, an
// assignment to anything but a plain identifier (a field, a slice element)
// is not a local and is refused.

// carry is the state a loop's body writes that outlives one iteration.
// Variables assigned or incremented are tracked by object. Maps written or
// deleted from are tracked by type, so a read through another variable
// holding the same map is caught too. When every write to maps of a type is
// m[k], with k the range key, each visit touches only its own entry, so
// reading any map of that type at [k] sees nothing an earlier visit wrote.
type carry struct {
	objs    map[types.Object]bool
	maps    []types.Type
	unkeyed []types.Type
	key     types.Object
}

// carriedState collects what loop's body writes that is declared outside
// it.
func carriedState(pass *analysis.Pass, loop *ast.RangeStmt) *carry {
	c := &carry{objs: map[types.Object]bool{}}
	if key, ok := loop.Key.(*ast.Ident); ok && key.Name != "_" && distinctKeys(pass, loop) {
		c.key = pass.TypesInfo.ObjectOf(key)
	}
	mapWrite := func(m, index ast.Expr) {
		t := pass.TypesInfo.TypeOf(m)
		if t == nil {
			return
		}
		c.maps = append(c.maps, t)
		if !c.isKey(pass, index) {
			c.unkeyed = append(c.unkeyed, t)
		}
	}
	ast.Inspect(loop.Body, func(node ast.Node) bool {
		var targets []ast.Expr
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			targets = stmt.Lhs
		case *ast.IncDecStmt:
			targets = []ast.Expr{stmt.X}
		case *ast.CallExpr:
			if isBuiltin(pass, stmt.Fun, "delete") && len(stmt.Args) == 2 {
				mapWrite(stmt.Args[0], stmt.Args[1])
			}
		}
		for _, target := range targets {
			if isMapIndex(pass, target) {
				index := ast.Unparen(target).(*ast.IndexExpr)
				mapWrite(index.X, index.Index)
				continue
			}
			if ident, ok := ast.Unparen(target).(*ast.Ident); ok && !declaredWithin(pass, loop, ident) {
				if obj := pass.TypesInfo.ObjectOf(ident); obj != nil {
					c.objs[obj] = true
				}
			}
			// A field written through a pointer may be read through
			// another entry that shares the pointer.
			if sel, ok := ast.Unparen(target).(*ast.SelectorExpr); ok && throughPointer(pass, sel) {
				if obj := pass.TypesInfo.ObjectOf(sel.Sel); obj != nil {
					c.objs[obj] = true
				}
			}
		}
		return true
	})
	return c
}

// distinctKeys reports whether loop's first variable differs on every
// visit: a map's keys do, maps.Values' elements need not.
func distinctKeys(pass *analysis.Pass, loop *ast.RangeStmt) bool {
	call, ok := ast.Unparen(loop.X).(*ast.CallExpr)
	if !ok {
		return true
	}
	return !isFunc(pass, call.Fun, "maps", "Values")
}

// throughPointer reports whether a selector chain dereferences a pointer
// anywhere, so the field it names may be shared.
func throughPointer(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	for {
		selection := pass.TypesInfo.Selections[sel]
		if selection == nil || selection.Indirect() {
			return true
		}
		next, ok := ast.Unparen(sel.X).(*ast.SelectorExpr)
		if !ok {
			return false
		}
		sel = next
	}
}

// isKey reports whether expr is the range key itself.
func (c *carry) isKey(pass *analysis.Pass, expr ast.Expr) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && c.key != nil && pass.TypesInfo.ObjectOf(ident) == c.key
}

// writtenMap reports whether t is the type of a map the body writes.
func (c *carry) writtenMap(t types.Type) bool {
	return slices.ContainsFunc(c.maps, func(m types.Type) bool { return types.Identical(m, t) })
}

// ownSlot reports whether index reads the range key's entry of a map type
// that is only ever written at the range key.
func (c *carry) ownSlot(pass *analysis.Pass, index *ast.IndexExpr) bool {
	t := pass.TypesInfo.TypeOf(index.X)
	if t == nil || !c.writtenMap(t) || !c.isKey(pass, index.Index) {
		return false
	}
	return !slices.ContainsFunc(c.unkeyed, func(u types.Type) bool { return types.Identical(u, t) })
}

// dirty reports whether expr reads state an earlier visit may have written.
func (c *carry) dirty(pass *analysis.Pass, expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if found {
			return false
		}
		if index, ok := node.(*ast.IndexExpr); ok && c.ownSlot(pass, index) {
			found = readsAny(pass, index.X, c.objs)
			return false
		}
		if ident, ok := node.(*ast.Ident); ok && c.objs[pass.TypesInfo.ObjectOf(ident)] {
			found = true
		}
		if value, ok := node.(ast.Expr); ok {
			tv, ok := pass.TypesInfo.Types[value]
			found = found || (ok && tv.IsValue() && c.writtenMap(tv.Type))
		}
		return !found
	})
	return found
}

// clean reports whether expr reads nothing carried.
func clean(pass *analysis.Pass, expr ast.Expr, carried *carry) bool {
	return carried == nil || !carried.dirty(pass, expr)
}

// readsAny reports whether expr mentions any object in objs.
func readsAny(pass *analysis.Pass, expr ast.Expr, objs map[types.Object]bool) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && objs[pass.TypesInfo.ObjectOf(ident)] {
			found = true
		}
		return !found
	})
	return found
}

// bodyAllowed walks loop's body against the closed allowlist. Nested
// if/block statements are expanded onto the worklist rather than recursed
// into, so the whole walk stays iterative.
func bodyAllowed(pass *analysis.Pass, loop *ast.RangeStmt, carried *carry) bool {
	work := append([]ast.Stmt{}, loop.Body.List...)
	for i := 0; i < len(work); i++ {
		next, ok := stmtAllowed(pass, loop, work[i], carried)
		if !ok {
			return false
		}
		work = append(work, next...)
	}
	return true
}

// stmtAllowed classifies one statement, returning the nested statements
// still to check.
func stmtAllowed(pass *analysis.Pass, loop *ast.RangeStmt, stmt ast.Stmt, carried *carry) ([]ast.Stmt, bool) {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		return nil, assignAllowed(pass, loop, stmt, carried)
	case *ast.IncDecStmt:
		return nil, incDecAllowed(pass, stmt, carried)
	case *ast.ExprStmt:
		return nil, deleteAllowed(pass, stmt, carried)
	case *ast.DeclStmt:
		return nil, declStmtAllowed(pass, loop, stmt, carried)
	case *ast.BranchStmt:
		return nil, stmt.Label == nil && (stmt.Tok == token.BREAK || stmt.Tok == token.CONTINUE)
	case *ast.ReturnStmt:
		return nil, returnAllowed(pass, stmt)
	case *ast.BlockStmt:
		return stmt.List, true
	case *ast.IfStmt:
		return ifAllowed(pass, stmt, carried)
	}
	return nil, false
}

// assignAllowed classifies an assignment against the map-write, integer
// accumulation, min/max reduction, boolean-constant, and local-declaration
// arms of the allowlist.
func assignAllowed(pass *analysis.Pass, loop *ast.RangeStmt, assign *ast.AssignStmt, carried *carry) bool {
	if mapWriteAssign(pass, assign, carried) {
		return true
	}
	if intAccumAssign(pass, assign, carried) {
		return true
	}
	if minMaxAssign(pass, assign, carried) {
		return true
	}
	if boolConstAssign(pass, assign) {
		return true
	}
	return localAssign(pass, loop, assign, carried)
}

// mapWriteAssign reports whether every LHS of assign indexes into a map,
// with keys and values that read no carried state.
func mapWriteAssign(pass *analysis.Pass, assign *ast.AssignStmt, carried *carry) bool {
	for _, lhs := range assign.Lhs {
		if !isMapIndex(pass, lhs) || !clean(pass, lhs.(*ast.IndexExpr).Index, carried) {
			return false
		}
	}
	for _, rhs := range assign.Rhs {
		if !clean(pass, rhs, carried) {
			return false
		}
	}
	return true
}

// isMapIndex reports whether expr is an index expression into a map.
func isMapIndex(pass *analysis.Pass, expr ast.Expr) bool {
	index, ok := expr.(*ast.IndexExpr)
	if !ok {
		return false
	}
	indexed := pass.TypesInfo.TypeOf(index.X)
	if indexed == nil {
		return false
	}
	_, mapped := indexed.Underlying().(*types.Map)
	return mapped
}

// intAccumAssign reports whether assign is x += e or x -= e for an integer
// x and an e that reads no carried state.
func intAccumAssign(pass *analysis.Pass, assign *ast.AssignStmt, carried *carry) bool {
	if assign.Tok != token.ADD_ASSIGN && assign.Tok != token.SUB_ASSIGN {
		return false
	}
	if len(assign.Lhs) != 1 {
		return false
	}
	return isIntIdent(pass, assign.Lhs[0]) && clean(pass, assign.Rhs[0], carried)
}

// isIntIdent reports whether expr is an identifier of integer type.
func isIntIdent(pass *analysis.Pass, expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	identType := pass.TypesInfo.TypeOf(ident)
	if identType == nil {
		return false
	}
	basic, ok := identType.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsInteger != 0
}

// minMaxAssign reports whether assign is x = min(x, e...) or
// x = max(x, e...) for an integer x, where each e reads no carried state.
func minMaxAssign(pass *analysis.Pass, assign *ast.AssignStmt, carried *carry) bool {
	if assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	ident, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || !isIntIdent(pass, ident) {
		return false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !(isBuiltin(pass, call.Fun, "min") || isBuiltin(pass, call.Fun, "max")) {
		return false
	}
	target := pass.TypesInfo.ObjectOf(ident)
	self := false
	for _, arg := range call.Args {
		if identDecl(pass, arg) == target {
			self = true
			continue
		}
		if !clean(pass, arg, carried) {
			return false
		}
	}
	return self
}

// boolConstAssign reports whether assign sets a boolean identifier to the
// constant true or false — half of the short-circuit shape.
func boolConstAssign(pass *analysis.Pass, assign *ast.AssignStmt) bool {
	if assign.Tok != token.ASSIGN && assign.Tok != token.DEFINE {
		return false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	ident, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || !isBoolIdent(pass, ident) {
		return false
	}
	return isBoolConst(pass, assign.Rhs[0])
}

// isBoolIdent reports whether ident has boolean type.
func isBoolIdent(pass *analysis.Pass, ident *ast.Ident) bool {
	identType := pass.TypesInfo.TypeOf(ident)
	if identType == nil {
		return false
	}
	basic, ok := identType.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsBoolean != 0
}

// isBoolConst reports whether expr is the predeclared constant true or
// false.
func isBoolConst(pass *analysis.Pass, expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	if ident.Name != "true" && ident.Name != "false" {
		return false
	}
	obj, ok := pass.TypesInfo.Uses[ident].(*types.Const)
	return ok && obj.Pkg() == nil
}

// localAssign reports whether assign declares or reassigns only identifiers
// declared inside loop's body, from pure values that read no carried state.
func localAssign(pass *analysis.Pass, loop *ast.RangeStmt, assign *ast.AssignStmt, carried *carry) bool {
	if assign.Tok != token.ASSIGN && assign.Tok != token.DEFINE {
		return false
	}
	entryWrite := false
	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			if ident.Name != "_" && !declaredWithin(pass, loop, ident) {
				return false
			}
			continue
		}
		root, ok := fieldWriteRoot(pass, loop, lhs)
		if !ok {
			return false
		}
		entryWrite = entryWrite || !declaredWithin(pass, loop, root)
	}
	for _, rhs := range assign.Rhs {
		if !pureExpr(pass, rhs) || !clean(pass, rhs, carried) {
			return false
		}
		if entryWrite && readsLoopLocal(pass, loop, rhs) {
			return false
		}
	}
	return true
}

// fieldWriteRoot accepts x.f.g = e where x is the range value (the entry
// the visit owns; pointers allowed) or a struct declared in the body (no
// pointer on the way, so it can't alias anything outside), and returns x.
// Slice and array element writes are refused: an index can name a slot
// another visit also writes.
func fieldWriteRoot(pass *analysis.Pass, loop *ast.RangeStmt, lhs ast.Expr) (*ast.Ident, bool) {
	indirect := false
	expr := ast.Unparen(lhs)
	for {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok {
			break
		}
		selection := pass.TypesInfo.Selections[sel]
		if selection == nil || selection.Kind() != types.FieldVal {
			return nil, false
		}
		indirect = indirect || selection.Indirect()
		expr = ast.Unparen(sel.X)
	}
	root, ok := expr.(*ast.Ident)
	if !ok || root == lhs {
		return nil, false
	}
	if declaredWithin(pass, loop, root) {
		return root, !indirect
	}
	value, ok := loop.Value.(*ast.Ident)
	if !ok || loop.Value == nil {
		value, ok = loop.Key.(*ast.Ident)
		ok = ok && !distinctKeys(pass, loop)
	}
	return root, ok && pass.TypesInfo.ObjectOf(root) == pass.TypesInfo.ObjectOf(value)
}

// readsLoopLocal reports whether expr reads the range key or anything
// declared in the body. A write to the entry must not: two keys can hold
// the same pointer, and then both visits must write the same value.
func readsLoopLocal(pass *analysis.Pass, loop *ast.RangeStmt, expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok || found {
			return !found
		}
		obj := pass.TypesInfo.ObjectOf(ident)
		if _, isVar := obj.(*types.Var); !isVar || obj.(*types.Var).IsField() {
			return true
		}
		if key, ok := loop.Key.(*ast.Ident); ok && obj == pass.TypesInfo.ObjectOf(key) && loop.Value != nil {
			found = true
		}
		found = found || declaredWithin(pass, loop, ident)
		return !found
	})
	return found
}

// declaredWithin reports whether ident's declaration lies inside loop's
// body, so it is a fresh per-iteration local rather than state carried
// across iterations.
func declaredWithin(pass *analysis.Pass, loop *ast.RangeStmt, ident *ast.Ident) bool {
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true
	}
	return obj.Pos() >= loop.Body.Pos() && obj.Pos() < loop.Body.End()
}

// incDecAllowed reports whether stmt increments or decrements a map index
// whose key reads no carried state, or an integer identifier.
func incDecAllowed(pass *analysis.Pass, stmt *ast.IncDecStmt, carried *carry) bool {
	if isMapIndex(pass, stmt.X) {
		return clean(pass, stmt.X.(*ast.IndexExpr).Index, carried)
	}
	return isIntIdent(pass, stmt.X)
}

// deleteAllowed reports whether stmt is a bare call to the builtin delete
// whose key reads no carried state.
func deleteAllowed(pass *analysis.Pass, stmt *ast.ExprStmt, carried *carry) bool {
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok || !isBuiltin(pass, call.Fun, "delete") || len(call.Args) != 2 {
		return false
	}
	return clean(pass, call.Args[1], carried)
}

// declStmtAllowed reports whether stmt is a var declaration of locals
// declared inside loop's body with pure initializers.
func declStmtAllowed(pass *analysis.Pass, loop *ast.RangeStmt, stmt *ast.DeclStmt, carried *carry) bool {
	gen, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.VAR {
		return false
	}
	for _, entry := range gen.Specs {
		named, ok := entry.(*ast.ValueSpec)
		if !ok {
			return false
		}
		for _, name := range named.Names {
			if name.Name != "_" && !declaredWithin(pass, loop, name) {
				return false
			}
		}
		for _, init := range named.Values {
			if !pureExpr(pass, init) || !clean(pass, init, carried) {
				return false
			}
		}
	}
	return true
}

// returnAllowed reports whether stmt returns only constant booleans (or
// nothing) — the other half of the short-circuit shape.
func returnAllowed(pass *analysis.Pass, stmt *ast.ReturnStmt) bool {
	for _, result := range stmt.Results {
		if !isBoolConst(pass, result) {
			return false
		}
	}
	return true
}

// ifAllowed reports whether ifStmt is the min/max if-form (a leaf, nothing
// further to check) or an if whose condition is pure and reads no carried
// state; in the latter case it returns the body and else statements.
func ifAllowed(pass *analysis.Pass, ifStmt *ast.IfStmt, carried *carry) ([]ast.Stmt, bool) {
	if minMaxIfForm(pass, ifStmt, carried) {
		return nil, true
	}
	if ifStmt.Init != nil {
		return nil, false
	}
	if !pureExpr(pass, ifStmt.Cond) || !clean(pass, ifStmt.Cond, carried) {
		return nil, false
	}
	next := append([]ast.Stmt{}, ifStmt.Body.List...)
	switch elseClause := ifStmt.Else.(type) {
	case nil:
	case *ast.BlockStmt:
		next = append(next, elseClause.List...)
	case *ast.IfStmt:
		next = append(next, elseClause)
	default:
		return nil, false
	}
	return next, true
}

// minMaxIfForm reports whether ifStmt is `if e > x { x = e }` or
// `if e < x { x = e }` (or the symmetric operand order), where e reads no
// carried state.
func minMaxIfForm(pass *analysis.Pass, ifStmt *ast.IfStmt, carried *carry) bool {
	if ifStmt.Init != nil || ifStmt.Else != nil {
		return false
	}
	if len(ifStmt.Body.List) != 1 {
		return false
	}
	binary, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	if binary.Op != token.GTR && binary.Op != token.LSS {
		return false
	}
	assign, ok := ifStmt.Body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	return matchesMinMaxUpdate(pass, binary, assign) && clean(pass, assign.Rhs[0], carried)
}

// matchesMinMaxUpdate reports whether assign sets the identifier on one
// side of binary to the expression on the other side.
func matchesMinMaxUpdate(pass *analysis.Pass, binary *ast.BinaryExpr, assign *ast.AssignStmt) bool {
	ident, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || !isIntIdent(pass, ident) {
		return false
	}
	target := pass.TypesInfo.ObjectOf(ident)
	left := identDecl(pass, binary.X)
	right := identDecl(pass, binary.Y)
	rhs := identDecl(pass, assign.Rhs[0])
	if rhs == nil || rhs == target {
		return false
	}
	if left == target && right == rhs {
		return true
	}
	return right == target && left == rhs
}

// identDecl returns the object expr resolves to when expr is an
// identifier, or nil otherwise.
func identDecl(pass *analysis.Pass, expr ast.Expr) types.Object {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return nil
	}
	return pass.TypesInfo.ObjectOf(ident)
}

// pureExpr reports whether expr contains no calls other than the builtins
// len, cap, min, and max.
func pureExpr(pass *analysis.Pass, expr ast.Expr) bool {
	pure := true
	ast.Inspect(expr, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return pure
		}
		allowed := false
		for _, name := range []string{"len", "cap", "min", "max"} {
			allowed = allowed || isBuiltin(pass, call.Fun, name)
		}
		pure = pure && allowed
		return pure
	})
	return pure
}
