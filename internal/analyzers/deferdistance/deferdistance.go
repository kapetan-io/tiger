// Package deferdistance enforces TS-L10: no defer inside a loop, and defers
// stay next to their acquisition.
//
// Two independent checks share one corpus, and both treat a func literal as
// a frame boundary: a defer runs when its own frame returns, so a defer
// inside a closure spawned by a loop queues nothing across iterations, and
// a release of something acquired outside the closure's frame has no local
// acquisition to sit next to. The loop half (TS-L10) fires on any DeferStmt
// lexically inside a ForStmt/RangeStmt body within the same frame — an
// unbounded queue of cleanup, which TS-S02 already forbids by construction.
// The distance half (TS-L10-distance) fires when a defer's call is a
// selector on a plain identifier (f.Close, tx.Rollback) and the statement
// in the same block that declared, assigned, or called a method on that
// identifier (mu.Lock before defer mu.Unlock) is not immediately followed
// by the defer, or by an `if err != nil`-style check on it. A deferred
// closure or a deferred package-level call never resolves to an
// identifier's acquisition statement, so both stay silent for the distance
// half; see the ts-l10 corpus known-miss case.
package deferdistance

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-L10: no defer inside a loop, and defers stay next to
// their acquisition.
var Analyzer = &analysis.Analyzer{
	Name: "deferdistance",
	Doc:  "TS-L10: no defer inside a loop, and defers stay next to their acquisition.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		checkLoopDefers(pass, file)
		checkDeferDistance(pass, file)
	}
	return nil, nil
}

// checkLoopDefers reports every DeferStmt lexically inside a
// ForStmt/RangeStmt body in the same frame. Entering a func literal saves
// the loop depth and starts from zero: the closure's defers resolve when
// its own frame returns, once per call, no matter how many frames the
// enclosing loop spawns.
func checkLoopDefers(pass *analysis.Pass, file *ast.File) {
	depth := 0
	saved := []int{}
	entered := []byte{}
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			last := entered[len(entered)-1]
			entered = entered[:len(entered)-1]
			switch last {
			case 'l':
				depth--
			case 'f':
				depth = saved[len(saved)-1]
				saved = saved[:len(saved)-1]
			}
			return true
		}
		mark := byte(0)
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			depth++
			mark = 'l'
		case *ast.FuncLit:
			saved = append(saved, depth)
			depth = 0
			mark = 'f'
		}
		entered = append(entered, mark)
		if depth > 0 {
			if deferStmt, ok := node.(*ast.DeferStmt); ok {
				reportLoop(pass, deferStmt)
			}
		}
		return true
	})
}

// checkDeferDistance visits every block in file and checks each direct
// DeferStmt child against the statements preceding it in that same block,
// tracking the innermost enclosing function body so a release of something
// acquired outside that frame stays silent.
func checkDeferDistance(pass *analysis.Pass, file *ast.File) {
	bodies := []*ast.BlockStmt{}
	entered := []bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			last := entered[len(entered)-1]
			entered = entered[:len(entered)-1]
			if last {
				bodies = bodies[:len(bodies)-1]
			}
			return true
		}
		body := functionBody(node)
		if body != nil {
			bodies = append(bodies, body)
		}
		entered = append(entered, body != nil)
		block, ok := node.(*ast.BlockStmt)
		if !ok || len(bodies) == 0 {
			return true
		}
		frame := bodies[len(bodies)-1]
		for index, stmt := range block.List {
			deferStmt, ok := stmt.(*ast.DeferStmt)
			if ok {
				checkDistance(pass, block.List[:index], deferStmt, frame)
			}
		}
		return true
	})
}

// functionBody returns node's body when node opens a new frame.
func functionBody(node ast.Node) *ast.BlockStmt {
	switch fn := node.(type) {
	case *ast.FuncDecl:
		return fn.Body
	case *ast.FuncLit:
		return fn.Body
	}
	return nil
}

// checkDistance reports deferStmt when its acquisition statement is not
// found immediately behind it (skipping only err-check IfStmts) among
// preceding, the statements before it in the same block. A target declared
// outside frame — a captured variable, a parameter, a package-level var
// with no acquisition-shaped statement here — was acquired by another frame
// and has nothing local to sit next to, so it stays silent.
func checkDistance(
	pass *analysis.Pass,
	preceding []ast.Stmt,
	deferStmt *ast.DeferStmt,
	frame *ast.BlockStmt,
) {
	target, ok := deferTarget(pass, deferStmt)
	if !ok {
		return
	}
	skipped := []ast.Stmt{}
	for index := len(preceding) - 1; index >= 0; index-- {
		stmt := preceding[index]
		names, found := acquisitionNames(stmt, target.Name)
		if found {
			if everyCheckReferences(skipped, names) {
				return
			}
			reportDistance(pass, deferStmt)
			return
		}
		skipped = append(skipped, stmt)
	}
	if !declaredWithin(pass, target, frame) {
		return
	}
	reportDistance(pass, deferStmt)
}

// declaredWithin reports whether target's declaration sits inside frame.
func declaredWithin(pass *analysis.Pass, target *ast.Ident, frame *ast.BlockStmt) bool {
	declared := pass.TypesInfo.Uses[target]
	if declared == nil {
		return true
	}
	return declared.Pos() >= frame.Pos() && declared.Pos() <= frame.End()
}

// everyCheckReferences reports whether every statement in skipped is an
// IfStmt whose condition references one of names.
func everyCheckReferences(skipped []ast.Stmt, names []string) bool {
	for _, stmt := range skipped {
		ifStmt, ok := stmt.(*ast.IfStmt)
		if !ok || !condReferencesAny(ifStmt.Cond, names) {
			return false
		}
	}
	return true
}

// deferTarget extracts the plain-identifier receiver of a deferred selector
// call (f.Close), reporting ok=false for closures, package-qualified calls,
// or any other call shape.
func deferTarget(pass *analysis.Pass, deferStmt *ast.DeferStmt) (*ast.Ident, bool) {
	selector, ok := deferStmt.Call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	if _, isPkg := pass.TypesInfo.Uses[ident].(*types.PkgName); isPkg {
		return nil, false
	}
	return ident, true
}

// acquisitionNames reports the identifiers stmt declares, assigns, or calls
// a method on — mu.Lock is the acquisition defer mu.Unlock releases — and
// whether target is one of them.
func acquisitionNames(stmt ast.Stmt, target string) ([]string, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if ok {
		names := identNames(assign.Lhs)
		return names, containsName(names, target)
	}
	if name, ok := methodCallReceiver(stmt); ok {
		return []string{name}, name == target
	}
	decl, ok := stmt.(*ast.DeclStmt)
	if !ok {
		return nil, false
	}
	genDecl, ok := decl.Decl.(*ast.GenDecl)
	if !ok {
		return nil, false
	}
	names := []string{}
	for _, item := range genDecl.Specs {
		declSpec, ok := item.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range declSpec.Names {
			names = append(names, name.Name)
		}
	}
	return names, containsName(names, target)
}

// methodCallReceiver extracts the plain-identifier receiver of a bare
// method-call statement (mu.Lock(), now.Freeze(t)).
func methodCallReceiver(stmt ast.Stmt) (string, bool) {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return "", false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	return ident.Name, true
}

// identNames returns the plain identifier names among exprs, skipping
// non-identifier targets such as a selector or index expression.
func identNames(exprs []ast.Expr) []string {
	names := []string{}
	for _, expr := range exprs {
		ident, ok := expr.(*ast.Ident)
		if ok {
			names = append(names, ident.Name)
		}
	}
	return names
}

// containsName reports whether target is present in names.
func containsName(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

// condReferencesAny reports whether cond mentions any identifier in names.
func condReferencesAny(cond ast.Expr, names []string) bool {
	found := false
	ast.Inspect(cond, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && containsName(names, ident.Name) {
			found = true
		}
		return true
	})
	return found
}

func reportLoop(pass *analysis.Pass, deferStmt *ast.DeferStmt) {
	pass.Report(analysis.Diagnostic{
		Pos:      deferStmt.Pos(),
		Category: "TS-L10",
		Message: "TS-L10: defer inside a loop queues cleanup without bound — move the loop body " +
			"into its own function so the defer runs once per call",
	})
}

func reportDistance(pass *analysis.Pass, deferStmt *ast.DeferStmt) {
	pass.Report(analysis.Diagnostic{
		Pos:      deferStmt.Pos(),
		Category: "TS-L10-distance",
		Message: "TS-L10: defer sits away from its acquisition — move this defer to the " +
			"statement immediately after the acquisition (or after its if err != nil check) " +
			"so acquire and release read together",
	})
}
