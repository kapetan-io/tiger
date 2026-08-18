// Package maporder enforces TS-T02: map iteration order never reaches an
// output.
//
// The spec's own enforcement text traces map order to a sink — "range over a
// map whose body appends or writes" — which is undecidable in general: any
// non-builtin call, however many frames deep, might eventually write what it
// receives somewhere order-visible, and proving it does not is a whole-program
// problem this analyzer does not have the machinery for. A heuristic sink
// trace would either miss real order dependence or fire on maybes, and a
// maybe on a blocking rule is a bug (the wave's correctness constraint).
//
// This analyzer inverts the rule instead: it bans every range over a map
// unless the loop body is provably order-insensitive, matched against a
// closed allowlist of shapes (map-to-map writes, commutative integer
// accumulation, delete, and pure per-element boolean short-circuits — plus
// the structurally neutral declarations and nested control flow those shapes
// need). Every other body is a finding, whether or not it is actually order
// dependent. This is exact in the direction that matters: every finding is
// either a real order dependence or a loop the closed allowlist cannot
// prove safe, and the compliant rewrite (collect the keys, sort them,
// range over the sorted slice) is always available. The cost is a small
// amount of sorted-iteration busywork on loops that happen to be safe in a
// shape the allowlist does not yet recognize — traded deliberately for zero
// false-positive risk on a blocking rule.
//
// The allowlist is closed and versioned with this analyzer: extending it to
// a new provably-safe shape is an analyzer change with its own corpus case,
// never a configuration knob.
package maporder

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const (
	msgAppend = "TS-T02: ranging over a map appends to a slice, so map iteration order " +
		"reaches an output — iterate the sorted keys instead: for _, k := range " +
		"slices.Sorted(maps.Keys(m))"
	msgCall = "TS-T02: ranging over a map calls a function whose behavior depends on call " +
		"order, so map iteration order reaches an output — iterate the sorted keys instead: " +
		"for _, k := range slices.Sorted(maps.Keys(m))"
	msgStringBuild = "TS-T02: ranging over a map builds a string, so map iteration order " +
		"reaches an output — iterate the sorted keys instead: for _, k := range " +
		"slices.Sorted(maps.Keys(m))"
	msgChannelSend = "TS-T02: ranging over a map sends on a channel, so map iteration order " +
		"reaches an output — iterate the sorted keys instead: for _, k := range " +
		"slices.Sorted(maps.Keys(m))"
	msgGeneric = "TS-T02: this map range's body is not in the closed order-insensitive " +
		"allowlist, so map iteration order can reach an output — iterate the sorted keys " +
		"instead: for _, k := range slices.Sorted(maps.Keys(m))"
)

// Analyzer enforces TS-T02: every range over a map is order-insensitive by
// construction, or it is a finding.
var Analyzer = &analysis.Analyzer{
	Name: "maporder",
	Doc:  "TS-T02: map iteration order never reaches an output.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			loop, ok := node.(*ast.RangeStmt)
			if ok {
				checkRange(pass, loop)
			}
			return true
		})
	}
	return nil, nil
}

// checkRange fires TS-T02 when loop ranges over a map and its body is not
// provably order-insensitive.
func checkRange(pass *analysis.Pass, loop *ast.RangeStmt) {
	if !isMapRange(pass, loop) {
		return
	}
	if bodyAllowed(pass, loop) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-T02",
		Message:  classifyMessage(pass, loop),
	})
}

// isMapRange reports whether loop's range target is a map (through any
// named type whose underlying type is a map).
func isMapRange(pass *analysis.Pass, loop *ast.RangeStmt) bool {
	rangeType := pass.TypesInfo.TypeOf(loop.X)
	if rangeType == nil {
		return false
	}
	_, mapped := rangeType.Underlying().(*types.Map)
	return mapped
}

// bodyAllowed walks loop's body against the closed allowlist. Nested
// if/block statements are expanded onto the worklist rather than recursed
// into, so the whole walk stays iterative.
func bodyAllowed(pass *analysis.Pass, loop *ast.RangeStmt) bool {
	work := append([]ast.Stmt{}, loop.Body.List...)
	for i := 0; i < len(work); i++ {
		switch stmt := work[i].(type) {
		case *ast.AssignStmt:
			if !assignAllowed(pass, loop, stmt) {
				return false
			}
		case *ast.IncDecStmt:
			if !incDecAllowed(pass, stmt) {
				return false
			}
		case *ast.ExprStmt:
			if !exprStmtAllowed(pass, stmt) {
				return false
			}
		case *ast.DeclStmt:
			if !declStmtAllowed(pass, loop, stmt) {
				return false
			}
		case *ast.BranchStmt:
			if !branchAllowed(stmt) {
				return false
			}
		case *ast.ReturnStmt:
			if !returnAllowed(pass, stmt) {
				return false
			}
		case *ast.BlockStmt:
			work = append(work, stmt.List...)
		case *ast.IfStmt:
			next, ok := ifAllowed(pass, stmt)
			if !ok {
				return false
			}
			work = append(work, next...)
		default:
			return false
		}
	}
	return true
}

// assignAllowed classifies an assignment against the map-write, integer
// accumulation, min/max reduction, boolean-constant, and local-declaration
// arms of the allowlist.
func assignAllowed(pass *analysis.Pass, loop *ast.RangeStmt, assign *ast.AssignStmt) bool {
	if mapWriteAssign(pass, assign) {
		return true
	}
	if intAccumAssign(pass, assign) {
		return true
	}
	if minMaxAssign(pass, assign) {
		return true
	}
	if boolConstAssign(pass, assign) {
		return true
	}
	return localAssign(pass, loop, assign)
}

// mapWriteAssign reports whether every LHS of assign indexes into a map —
// category 1: writes into another map, including set-building and
// accumulation into a map value.
func mapWriteAssign(pass *analysis.Pass, assign *ast.AssignStmt) bool {
	for _, lhs := range assign.Lhs {
		if !isMapIndex(pass, lhs) {
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
// x — category 2's commutative accumulation.
func intAccumAssign(pass *analysis.Pass, assign *ast.AssignStmt) bool {
	if assign.Tok != token.ADD_ASSIGN && assign.Tok != token.SUB_ASSIGN {
		return false
	}
	if len(assign.Lhs) != 1 {
		return false
	}
	return isIntIdent(pass, assign.Lhs[0])
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

// minMaxAssign reports whether assign is x = min(x, v) or x = max(x, v) for
// an integer x — category 2's min/max reduction form.
func minMaxAssign(pass *analysis.Pass, assign *ast.AssignStmt) bool {
	if assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	ident, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || !isIntIdent(pass, ident) {
		return false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !isMinMaxBuiltin(pass, call) {
		return false
	}
	return argContainsIdent(pass, call, ident)
}

// isMinMaxBuiltin reports whether call invokes the builtin min or max.
func isMinMaxBuiltin(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return false
	}
	return builtin.Name() == "min" || builtin.Name() == "max"
}

// argContainsIdent reports whether one of call's arguments is ident,
// resolved by object identity.
func argContainsIdent(pass *analysis.Pass, call *ast.CallExpr, ident *ast.Ident) bool {
	obj := pass.TypesInfo.ObjectOf(ident)
	for _, arg := range call.Args {
		argIdent, ok := arg.(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(argIdent) == obj {
			return true
		}
	}
	return false
}

// boolConstAssign reports whether assign sets a boolean identifier to the
// constant true or false — half of category 4's short-circuit shape.
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
// declared inside loop's body, with an initializer free of non-builtin
// calls — category 5's structurally neutral locals.
func localAssign(pass *analysis.Pass, loop *ast.RangeStmt, assign *ast.AssignStmt) bool {
	if assign.Tok != token.ASSIGN && assign.Tok != token.DEFINE {
		return false
	}
	for _, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		if !declaredWithin(pass, loop, ident) {
			return false
		}
	}
	for _, rhs := range assign.Rhs {
		if !pureExpr(pass, rhs) {
			return false
		}
	}
	return true
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
// (category 1) or an integer identifier (category 2).
func incDecAllowed(pass *analysis.Pass, stmt *ast.IncDecStmt) bool {
	if isMapIndex(pass, stmt.X) {
		return true
	}
	return isIntIdent(pass, stmt.X)
}

// exprStmtAllowed reports whether stmt is a bare call to the builtin
// delete — category 3.
func exprStmtAllowed(pass *analysis.Pass, stmt *ast.ExprStmt) bool {
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok && builtin.Name() == "delete"
}

// declStmtAllowed reports whether stmt is a var declaration of locals
// declared inside loop's body with pure initializers — category 5.
func declStmtAllowed(pass *analysis.Pass, loop *ast.RangeStmt, stmt *ast.DeclStmt) bool {
	gen, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.VAR {
		return false
	}
	for _, entry := range gen.Specs {
		named, ok := entry.(*ast.ValueSpec)
		if !ok {
			return false
		}
		if !specAllowed(pass, loop, named) {
			return false
		}
	}
	return true
}

// specAllowed reports whether every name named declares is local to loop's
// body and every initializer is pure.
func specAllowed(pass *analysis.Pass, loop *ast.RangeStmt, named *ast.ValueSpec) bool {
	for _, name := range named.Names {
		if name.Name == "_" {
			continue
		}
		if !declaredWithin(pass, loop, name) {
			return false
		}
	}
	for _, init := range named.Values {
		if !pureExpr(pass, init) {
			return false
		}
	}
	return true
}

// branchAllowed reports whether stmt is an unlabeled break or continue.
func branchAllowed(stmt *ast.BranchStmt) bool {
	if stmt.Label != nil {
		return false
	}
	return stmt.Tok == token.BREAK || stmt.Tok == token.CONTINUE
}

// returnAllowed reports whether stmt returns only constant booleans (or
// nothing) — the other half of category 4's short-circuit shape.
func returnAllowed(pass *analysis.Pass, stmt *ast.ReturnStmt) bool {
	for _, result := range stmt.Results {
		if !isBoolConst(pass, result) {
			return false
		}
	}
	return true
}

// ifAllowed reports whether ifStmt is the min/max if-form (a leaf, nothing
// further to check) or a structurally neutral if whose condition is pure;
// in the latter case it returns the body and else statements to enqueue.
func ifAllowed(pass *analysis.Pass, ifStmt *ast.IfStmt) ([]ast.Stmt, bool) {
	if minMaxIfForm(pass, ifStmt) {
		return nil, true
	}
	if ifStmt.Init != nil {
		return nil, false
	}
	if !pureExpr(pass, ifStmt.Cond) {
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

// minMaxIfForm reports whether ifStmt is `if v > x { x = v }` or
// `if v < x { x = v }` (or the symmetric operand order) — the if-form half
// of category 2's min/max reduction.
func minMaxIfForm(pass *analysis.Pass, ifStmt *ast.IfStmt) bool {
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
	return matchesMinMaxUpdate(pass, binary, assign)
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
	if rhs == nil {
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
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	return pass.TypesInfo.ObjectOf(ident)
}

// pureExpr reports whether expr contains no calls other than the builtins
// len, cap, min, and max — the conservative purity category 4 and 5 require
// of conditions and initializers.
func pureExpr(pass *analysis.Pass, expr ast.Expr) bool {
	pure := true
	ast.Inspect(expr, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && !isAllowedPureCall(pass, call) {
			pure = false
		}
		return true
	})
	return pure
}

// isAllowedPureCall reports whether call invokes one of the builtins a
// pure expression may still call: len, cap, min, max.
func isAllowedPureCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return false
	}
	switch builtin.Name() {
	case "len", "cap", "min", "max":
		return true
	default:
		return false
	}
}

// classifyMessage picks the finding message that best names what loop's
// body does that lets order escape, falling back to a generic message when
// no specific shape is found.
func classifyMessage(pass *analysis.Pass, loop *ast.RangeStmt) string {
	found := ""
	ast.Inspect(loop.Body, func(node ast.Node) bool {
		if found != "" {
			return false
		}
		switch signal := node.(type) {
		case *ast.SendStmt:
			found = msgChannelSend
		case *ast.BinaryExpr:
			if signal.Op == token.ADD && isStringType(pass, signal) {
				found = msgStringBuild
			}
		case *ast.CallExpr:
			if isAppendCall(pass, signal) {
				found = msgAppend
			} else if isPlainCall(pass, signal) {
				found = msgCall
			}
		}
		return found == ""
	})
	if found == "" {
		return msgGeneric
	}
	return found
}

// isAppendCall reports whether call invokes the builtin append.
func isAppendCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok && builtin.Name() == "append"
}

// isPlainCall reports whether call invokes anything other than the
// allowlisted builtins len, cap, min, max, delete, and append.
func isPlainCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return true
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return true
	}
	switch builtin.Name() {
	case "len", "cap", "min", "max", "delete", "append":
		return false
	default:
		return true
	}
}

// isStringType reports whether expr has string type.
func isStringType(pass *analysis.Pass, expr ast.Expr) bool {
	exprType := pass.TypesInfo.TypeOf(expr)
	if exprType == nil {
		return false
	}
	basic, ok := exprType.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}
