// Package deferdistance enforces TS-L10: no defer inside a loop, and defers
// stay next to their acquisition.
//
// Two independent checks share one corpus. The loop half (TS-L10) fires on
// any DeferStmt lexically inside a ForStmt/RangeStmt body — an unbounded
// queue of cleanup, which TS-S02 already forbids by construction. The
// distance half (TS-L10-distance) fires when a defer's call is a selector on
// a plain identifier (f.Close, tx.Rollback) and the statement in the same
// block that declared or assigned that identifier is not immediately
// followed by the defer, or by an `if err != nil`-style check on it. A
// deferred closure or a deferred package-level call never resolves to an
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
// ForStmt/RangeStmt body, tracking loop depth across the walk.
func checkLoopDefers(pass *analysis.Pass, file *ast.File) {
	depth := 0
	entered := []bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			last := entered[len(entered)-1]
			entered = entered[:len(entered)-1]
			if last {
				depth--
			}
			return true
		}
		_, isFor := node.(*ast.ForStmt)
		_, isRange := node.(*ast.RangeStmt)
		loop := isFor || isRange
		if loop {
			depth++
		}
		entered = append(entered, loop)
		if depth > 0 {
			if deferStmt, ok := node.(*ast.DeferStmt); ok {
				reportLoop(pass, deferStmt)
			}
		}
		return true
	})
}

// checkDeferDistance visits every block in file and checks each direct
// DeferStmt child against the statements preceding it in that same block.
func checkDeferDistance(pass *analysis.Pass, file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for index, stmt := range block.List {
			deferStmt, ok := stmt.(*ast.DeferStmt)
			if ok {
				checkDistance(pass, block.List[:index], deferStmt)
			}
		}
		return true
	})
}

// checkDistance reports deferStmt when its acquisition statement is not
// found immediately behind it (skipping only err-check IfStmts) among
// preceding, the statements before it in the same block.
func checkDistance(pass *analysis.Pass, preceding []ast.Stmt, deferStmt *ast.DeferStmt) {
	target, ok := deferTarget(pass, deferStmt)
	if !ok {
		return
	}
	skipped := []ast.Stmt{}
	for index := len(preceding) - 1; index >= 0; index-- {
		stmt := preceding[index]
		names, found := assignedNames(stmt, target)
		if found {
			if everyCheckReferences(skipped, names) {
				return
			}
			reportDistance(pass, deferStmt)
			return
		}
		skipped = append(skipped, stmt)
	}
	reportDistance(pass, deferStmt)
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
func deferTarget(pass *analysis.Pass, deferStmt *ast.DeferStmt) (string, bool) {
	selector, ok := deferStmt.Call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	if _, isPkg := pass.TypesInfo.Uses[ident].(*types.PkgName); isPkg {
		return "", false
	}
	return ident.Name, true
}

// assignedNames reports the identifiers stmt declares or assigns, and
// whether target is one of them.
func assignedNames(stmt ast.Stmt, target string) ([]string, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if ok {
		names := identNames(assign.Lhs)
		return names, containsName(names, target)
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
