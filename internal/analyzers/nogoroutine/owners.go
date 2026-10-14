package nogoroutine

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/ownership"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/stopcase"
)

// groupTypes are the types whose Go method starts a goroutine and whose Wait
// method joins it.
var groupTypes = map[string]bool{
	"sync.WaitGroup":                   true,
	"golang.org/x/sync/errgroup.Group": true,
}

// startMethods are the group methods that start a goroutine.
var startMethods = map[string]bool{"Go": true, "TryGo": true}

// owners judges the goroutines a package starts through groups.
type owners struct {
	pass  *analysis.Pass
	index *ownership.Index
	stops *stopcase.Finder
	waits map[*types.Var][]wait
}

// wait is one Wait call on a group.
type wait struct {
	decl *ast.FuncDecl
}

// newOwners indexes every Wait on a group in the package that sits on the
// success path of the function holding it.
func newOwners(pass *analysis.Pass) *owners {
	stops := stopcase.New(pass)
	found := &owners{pass: pass, index: stops.Index(), stops: stops, waits: map[*types.Var][]wait{}}
	for _, file := range pass.Files {
		var expired []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				expired = expired[:len(expired)-1]
				return false
			}
			if call, ok := node.(*ast.CallExpr); ok && !found.inExpiredPath(expired) {
				found.recordWait(call)
			}
			expired = append(expired, node)
			return true
		})
	}
	return found
}

// inExpiredPath reports whether the innermost select case around a node
// receives from ctx.Done(): the early return of a function whose caller gave
// up waiting.
func (o *owners) inExpiredPath(ancestors []ast.Node) bool {
	for i := len(ancestors) - 1; i >= 0; i-- {
		if clause, ok := ancestors[i].(*ast.CommClause); ok {
			return clause.Comm != nil && o.stops.ContextCase(clause.Comm)
		}
	}
	return false
}

// recordWait notes call as a Wait on its group.
func (o *owners) recordWait(call *ast.CallExpr) {
	recv, ok := o.groupMethod(call, "Wait")
	if !ok {
		return
	}
	if obj := o.index.Object(recv); obj != nil {
		o.waits[obj] = append(o.waits[obj], wait{decl: o.index.DeclOf(call)})
	}
}

// groupMethod returns the receiver expression of a call to a method of a
// group type, when the method is named by want, or is one of startMethods if
// want is empty.
func (o *owners) groupMethod(call *ast.CallExpr, want string) (ast.Expr, bool) {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	fn, ok := o.pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok {
		return nil, false
	}
	if want == "" && !startMethods[fn.Name()] {
		return nil, false
	}
	if want != "" && fn.Name() != want {
		return nil, false
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return nil, false
	}
	typ := recv.Type()
	if ptr, isPtr := typ.(*types.Pointer); isPtr {
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return nil, false
	}
	if !groupTypes[named.Obj().Pkg().Path()+"."+named.Obj().Name()] {
		return nil, false
	}
	return selector.X, true
}

// checkStart reports TS-C02 when call starts a goroutine through a group that
// nothing waits on, and reports whether call starts a goroutine at all.
func (o *owners) checkStart(call *ast.CallExpr) bool {
	recv, ok := o.groupMethod(call, "")
	if !ok {
		return false
	}
	if !o.waited(recv, call) {
		o.pass.Report(analysis.Diagnostic{
			Pos:      call.Pos(),
			Category: "TS-C02",
			Message:  fmt.Sprintf(msgNoWaitFmt, methodName(call)),
		})
	}
	return true
}

// methodName names the group method call invokes, as "wg.Go" would read.
func methodName(call *ast.CallExpr) string {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return "call"
	}
	return selector.Sel.Name + " call"
}

// waited reports whether a Wait on the group recv names is reached for a
// start at call. A group the code cannot name, such as a call result, has no
// Wait to find. A parameter is the caller's to wait on.
func (o *owners) waited(recv ast.Expr, call *ast.CallExpr) bool {
	group := o.index.Object(recv)
	if group == nil {
		return false
	}
	if o.index.IsParameter(group) {
		return true
	}
	use := o.index.DeclOf(call)
	for _, site := range o.waits[group] {
		if o.index.Reaches(group, ownership.Link{Actor: site.decl, Reader: use}) {
			return true
		}
	}
	return false
}
