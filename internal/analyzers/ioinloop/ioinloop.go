// Package ioinloop enforces TS-M10: no IO call inside a loop body.
//
// A call is IO when its callee — function or method — resolves through
// pass.TypesInfo to a *types.Func whose declaring package is on the
// allowlist, never by name or by the interface it happens to satisfy: an
// io.Writer implementer includes the in-memory bytes.Buffer, and flagging
// it would break the exact-tier invariant (blocking findings are true by
// construction). Within an allowlist package, a method on a map-, slice-,
// or basic-underlying receiver is exempt for the same reason: a pure
// container or value holds no connection or descriptor, so the call is an
// in-memory operation (http.Header.Set — a map write — was a trial false
// positive on the git-server pin). The seed allowlist — os (filesystem),
// net and net/http (sockets and HTTP transport), database/sql (database
// drivers), and syscall (raw system calls) — is extensible per repo
// through the -ioinloop.packages flag, appended to the seed.
//
// Init, Cond, Post, and a RangeStmt's X each run once per loop, not once
// per iteration, so IO there is deliberately unflagged — this is what
// keeps idiomatic cursor-advance conditions (for rows.Next()) out of scope.
// A FuncLit is a frame boundary, consistent with deferdistance: IO inside a
// closure defined in the loop is not attributed to the loop.
//
// //tiger:batched waives TS-M10 in a loop's own body only: an inner loop
// under an outer-only-annotated directive still fires, so accounting stays
// per decision. Directive validation (a known verb, a reason) stays solely
// in the directives analyzer; this package parses but never reports
// TS-L09, so the TS-L09-escape standing advisory is never suppressed by
// consumption.
package ioinloop

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/attach"
)

// seedPackages are the stdlib import paths whose operations are
// unconditionally IO, independent of receiver type.
var seedPackages = []string{"os", "net", "net/http", "database/sql", "syscall"}

const msg = "TS-M10: this call resolves to a package on the IO allowlist and runs once per " +
	"loop iteration — hoist it above the loop, or mark the loop //tiger:batched <reason> when " +
	"per-item IO is what the outside world forces"

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

// Analyzer enforces TS-M10: no IO call inside a loop body.
var Analyzer = newAnalyzer()

func run(pass *analysis.Pass) (any, error) {
	allowed := allowlist()
	for _, file := range pass.Files {
		w := walker{pass: pass, file: file, allowed: allowed}
		w.walk(file, nil)
	}
	return nil, nil
}

// allowlist merges seedPackages with the per-repo -packages flag.
func allowlist() map[string]bool {
	allowed := map[string]bool{}
	for _, path := range seedPackages {
		allowed[path] = true
	}
	for path := range strings.SplitSeq(packagesFlag, ",") {
		path = strings.TrimSpace(path)
		if path != "" {
			allowed[path] = true
		}
	}
	return allowed
}

// walker holds the state that stays fixed for one pass.Files entry —
// pass, file, and the merged IO allowlist — so walk/walkOpt only need to
// carry what changes as they descend: root and frame.
type walker struct {
	pass    *analysis.Pass
	file    *ast.File
	allowed map[string]bool
}

// walk restarts ast.Inspect at root, carrying frame — the //tiger:batched
// state of every same-frame loop currently open, innermost last. A
// ForStmt/RangeStmt pushes its own annotated flag only around Body:
// Init/Cond/Post/X are walked under the unchanged frame, since they run
// once per loop rather than once per iteration. A FuncLit resets frame to
// nil, starting a new accounting frame. Both cases return false to stop
// ast.Inspect's automatic descent, since the children are walked manually
// with the correct frame.
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
			// Full-slice expression keeps sibling loops that share a
			// parent frame from aliasing one another's backing array.
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

// walkOpt calls walk unless node is the true nil an unset Init/Cond/Post
// carries (ForStmt/RangeStmt bodies are never nil for parsed source, so no
// typed-nil pointer ever reaches here as a node).
func (w *walker) walkOpt(node ast.Node, frame []bool) {
	if node == nil {
		return
	}
	w.walk(node, frame)
}

// annotated reports whether loop carries a well-formed //tiger:batched
// directive. Validation (a known verb, a reason) is the directives
// analyzer's job; a malformed attempt simply does not attach here.
func (w *walker) annotated(loop ast.Node) bool {
	_, ok := attach.Directive(w.pass.Fset, w.file, loop, "batched")
	return ok
}

// isIO reports whether call resolves to a *types.Func declared in a
// package on w.allowed. A method whose receiver's underlying type is a
// map, slice, or basic value is exempt even there: a pure container or
// value cannot hold a connection or descriptor, so its methods are
// in-memory (http.Header.Set on the wire path is the archetype — a map
// write, not IO). IO lives behind struct-backed receivers (os.File,
// http.Client, sql.DB) and package-level functions.
func (w *walker) isIO(call *ast.CallExpr) bool {
	fn := w.calleeFunc(call)
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	return w.allowed[fn.Pkg().Path()] && !containerMethod(fn)
}

// containerMethod reports whether fn is a method on a receiver whose
// underlying type — behind any pointer — is a map, slice, or basic value.
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

// calleeFunc resolves call's callee to a *types.Func through
// pass.TypesInfo, never by name. A method call resolves through
// Selections to the func actually bound to the receiver's type — for an
// interface-typed receiver that is the interface's own method declaration,
// carrying the interface's declaring package, not any concrete
// implementer's. A plain identifier call (a package-level function, a
// package-qualified call such as os.ReadFile) resolves through Uses.
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
