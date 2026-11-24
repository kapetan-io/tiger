// Package ioinloop enforces TS-M10: no IO call inside a loop body.
package ioinloop

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/attach"
	"github.com/kapetan-io/tiger/internal/config"
)

var seedPackages = []string{"os", "net", "net/http", "database/sql", "syscall"}

const msg = "TS-M10: this call does IO (network, disk, or database) and runs once per loop " +
	"iteration — move it above the loop, or if each item genuinely needs its own IO, mark " +
	"the loop //tiger:batched <reason>"

var packagesFlag string

func newAnalyzer() *analysis.Analyzer {
	built := &analysis.Analyzer{
		Name: "ioinloop",
		Doc:  "TS-M10: no IO call inside a loop body.",
		Run:  run,
	}
	built.Flags.StringVar(&packagesFlag, "packages", "",
		"comma-separated import paths appended to the seed IO allowlist")
	return built
}

var Analyzer = newAnalyzer()

func run(pass *analysis.Pass) (any, error) {
	allowed := allowlist(pass)
	for _, file := range pass.Files {
		w := walker{pass: pass, file: file, allowed: allowed}
		w.walk(file, nil)
	}
	return nil, nil
}

func allowlist(pass *analysis.Pass) map[string]bool {
	allowed := map[string]bool{}
	for _, path := range seedPackages {
		allowed[path] = true
	}
	paths := strings.Split(packagesFlag, ",")
	paths = append(paths, config.Values(
		pass.Pkg.Path(), config.Key{Analyzer: "ioinloop", Flag: "packages"},
	)...)
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path != "" {
			allowed[path] = true
		}
	}
	return allowed
}

type walker struct {
	pass    *analysis.Pass
	file    *ast.File
	allowed map[string]bool
}

func (w *walker) walk(root ast.Node, frame []bool) {
	ast.Inspect(root, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			w.walkOpt(n.Body, nil)
			return false
		case *ast.ForStmt:
			w.walkOpt(n.Init, frame)
			w.walkOpt(n.Cond, frame)
			w.walkOpt(n.Post, frame)
			w.walkOpt(n.Body, append(frame[:len(frame):len(frame)], w.annotated(n)))
			return false
		case *ast.RangeStmt:
			w.walkOpt(n.X, frame)
			w.walkOpt(n.Body, append(frame[:len(frame):len(frame)], w.annotated(n)))
			return false
		case *ast.CallExpr:
			if len(frame) > 0 && !frame[len(frame)-1] && w.isIO(n) {
				w.pass.Report(analysis.Diagnostic{Pos: n.Pos(), Category: "TS-M10", Message: msg})
			}
		}
		return true
	})
}

func (w *walker) walkOpt(node ast.Node, frame []bool) {
	if node == nil {
		return
	}
	w.walk(node, frame)
}

func (w *walker) annotated(loop ast.Node) bool {
	_, ok := attach.Directive(w.pass.Fset, w.file, loop, "batched")
	return ok
}

func (w *walker) isIO(call *ast.CallExpr) bool {
	fn := w.calleeFunc(call)
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	return w.allowed[fn.Pkg().Path()] && !containerMethod(fn)
}

func containerMethod(fn *types.Func) bool {
	recv := fn.Signature().Recv()
	if recv == nil {
		return false
	}
	target := recv.Type()
	if pointer, ok := target.Underlying().(*types.Pointer); ok {
		target = pointer.Elem()
	}
	switch target.Underlying().(type) {
	case *types.Map, *types.Slice, *types.Basic:
		return true
	default:
		return false
	}
}

func (w *walker) calleeFunc(call *ast.CallExpr) *types.Func {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if f, ok := w.pass.TypesInfo.Uses[fun].(*types.Func); ok {
			return f
		}
		return nil
	case *ast.SelectorExpr:
		if sel, ok := w.pass.TypesInfo.Selections[fun]; ok {
			if f, ok := sel.Obj().(*types.Func); ok {
				return f
			}
			return nil
		}
		if f, ok := w.pass.TypesInfo.Uses[fun.Sel].(*types.Func); ok {
			return f
		}
		return nil
	default:
		return nil
	}
}
