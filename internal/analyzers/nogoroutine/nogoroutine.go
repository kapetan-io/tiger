// Package nogoroutine enforces TS-C02 and TS-C09: no bare goroutine spawns
// outside supervisors.
//
// Every GoStmt is a finding unless the enclosing top-level function is on
// the -supervisors allowlist. A spawn lexically inside a for or range loop
// body is the reactive anti-pattern TS-C09 warns about — work started in
// direct response to each event or item, with no back pressure. Any other
// unowned spawn is TS-C02: a goroutine with no documented owner and no
// lifetime tied to a context.
package nogoroutine

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-C02 and TS-C09: no bare goroutine spawns outside
// supervisors.
var Analyzer = newAnalyzer()

var supervisors string

func newAnalyzer() *analysis.Analyzer {
	built := &analysis.Analyzer{
		Name: "nogoroutine",
		Doc:  "TS-C02 and TS-C09: no bare goroutine spawns outside supervisors.",
		Run:  run,
	}
	built.Flags.StringVar(&supervisors, "supervisors", "",
		"comma-separated qualified names (pkgpath.Func or pkgpath.Type.Method) "+
			"allowed to start goroutines directly")
	return built
}

const (
	msgUnowned = "TS-C02: bare goroutine has no owner, no documented exit condition, and no " +
		"lifetime tied to a context — start it through errgroup.Group.Go or a supervisor " +
		"function on the -supervisors allowlist"
	msgReactive = "TS-C09: goroutine spawned per loop iteration reacts directly to each item " +
		"instead of running at its own pace — append the work to a bounded queue and drain it " +
		"from a supervisor"
)

func run(pass *analysis.Pass) (any, error) {
	allowed := allowlist()
	for _, file := range pass.Files {
		var ancestors []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				ancestors = ancestors[:len(ancestors)-1]
				return false
			}
			if spawn, ok := node.(*ast.GoStmt); ok {
				check(pass, spawn, ancestors, allowed)
			}
			ancestors = append(ancestors, node)
			return true
		})
	}
	return nil, nil
}

// check reports a finding for spawn unless its owning function is
// allowlisted, choosing TS-C09 over TS-C02 when spawn sits inside a loop.
func check(pass *analysis.Pass, spawn *ast.GoStmt, ancestors []ast.Node, allowed map[string]bool) {
	owner := enclosingFunc(ancestors)
	if owner == nil {
		return
	}
	if allowed[qualifiedName(pass, owner)] {
		return
	}
	if insideLoop(ancestors) {
		pass.Report(analysis.Diagnostic{Pos: spawn.Pos(), Category: "TS-C09", Message: msgReactive})
		return
	}
	pass.Report(analysis.Diagnostic{Pos: spawn.Pos(), Category: "TS-C02", Message: msgUnowned})
}

// enclosingFunc returns the nearest FuncDecl among ancestors: the top-level
// function a GoStmt sits in, even when the GoStmt is nested inside a
// FuncLit closure.
func enclosingFunc(ancestors []ast.Node) *ast.FuncDecl {
	for i := len(ancestors) - 1; i >= 0; i-- {
		if decl, ok := ancestors[i].(*ast.FuncDecl); ok {
			return decl
		}
	}
	return nil
}

// insideLoop reports whether any ancestor is a for or range loop.
func insideLoop(ancestors []ast.Node) bool {
	for _, node := range ancestors {
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	return false
}

// qualifiedName renders decl as "pkgpath.Func" or, for a method,
// "pkgpath.Type.Method" — the shape the -supervisors flag names.
func qualifiedName(pass *analysis.Pass, decl *ast.FuncDecl) string {
	path := pass.Pkg.Path()
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return path + "." + decl.Name.Name
	}
	return path + "." + receiverName(decl.Recv.List[0].Type) + "." + decl.Name.Name
}

// receiverName extracts a method receiver's type name, unwrapping a
// pointer receiver.
func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// allowlist parses the -supervisors flag into a set of qualified names.
func allowlist() map[string]bool {
	names := map[string]bool{}
	for _, name := range strings.Split(supervisors, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			names[name] = true
		}
	}
	return names
}
