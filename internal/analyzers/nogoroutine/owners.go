package nogoroutine

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/ownership"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/stopcase"
	"github.com/kapetan-io/tiger/internal/worklist"
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
			if sel, ok := node.(*ast.SelectorExpr); ok && !found.inExpiredPath(expired) {
				found.recordWait(sel)
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

// recordWait notes sel as a Wait on its group.
func (o *owners) recordWait(sel *ast.SelectorExpr) {
	group, ok := o.groupMethod(sel, "Wait")
	if ok && group != nil {
		o.waits[group] = append(o.waits[group], wait{decl: o.index.DeclOf(sel)})
	}
}

// groupMethod returns the group a selector picks a method of, when the method
// is named by want, or is one of startMethods if want is empty. The group is
// the variable the selector names, or the embedded field the method is
// promoted from. It is nil when the code names the group by an expression
// tiger cannot follow, such as a call result.
func (o *owners) groupMethod(sel *ast.SelectorExpr, want string) (*types.Var, bool) {
	fn, ok := o.pass.TypesInfo.Uses[sel.Sel].(*types.Func)
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
	typ := types.Unalias(recv.Type())
	if ptr, isPtr := typ.(*types.Pointer); isPtr {
		typ = types.Unalias(ptr.Elem())
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return nil, false
	}
	if !groupTypes[named.Obj().Pkg().Path()+"."+named.Obj().Name()] {
		return nil, false
	}
	if embedded := o.embeddedField(sel); embedded != nil {
		return embedded, true
	}
	return o.unalias(o.index.Object(sel.X), sel), true
}

// aliasHopsMax bounds how many local aliases of a group are followed.
const aliasHopsMax = 8

const _ = uint(worklist.NodesMax - aliasHopsMax)

// unalias follows a local variable assigned once from another variable, such
// as group := &a.wg, to the variable it names, so a Wait on a.wg is a Wait on
// group.
func (o *owners) unalias(group *types.Var, at ast.Node) *types.Var {
	decl := o.index.DeclOf(at)
	for range aliasHopsMax {
		if group == nil || group.IsField() || o.index.IsParameter(group) {
			return group
		}
		if group.Parent() == group.Pkg().Scope() {
			return group
		}
		defs := o.index.Definitions(group, decl)
		if len(defs) != 1 {
			return group
		}
		source := ast.Unparen(defs[0])
		if address, ok := source.(*ast.UnaryExpr); ok && address.Op == token.AND {
			source = address.X
		}
		next := o.index.Object(source)
		if next == nil {
			return group
		}
		group = next
	}
	return group
}

// embeddedField returns the field a promoted method is reached through, or
// nil when the selector names the group directly.
func (o *owners) embeddedField(sel *ast.SelectorExpr) *types.Var {
	selection := o.pass.TypesInfo.Selections[sel]
	if selection == nil || len(selection.Index()) < 2 {
		return nil
	}
	typ := selection.Recv()
	var field *types.Var
	path := selection.Index()
	for i := 0; i < len(path)-1; i++ {
		if ptr, isPtr := types.Unalias(typ).Underlying().(*types.Pointer); isPtr {
			typ = ptr.Elem()
		}
		fields, ok := types.Unalias(typ).Underlying().(*types.Struct)
		if !ok {
			return nil
		}
		field = fields.Field(path[i])
		typ = field.Type()
	}
	if field == nil {
		return nil
	}
	return field.Origin()
}

// checkStart reports TS-C02 when sel starts a goroutine through a group that
// nothing waits on, and reports whether sel starts a goroutine at all.
func (o *owners) checkStart(sel *ast.SelectorExpr) bool {
	group, ok := o.groupMethod(sel, "")
	if !ok {
		return false
	}
	if !o.waited(group, sel) {
		o.pass.Report(analysis.Diagnostic{
			Pos:      sel.Pos(),
			Category: "TS-C02",
			Message:  fmt.Sprintf(msgNoWaitFmt, sel.Sel.Name+" call"),
		})
	}
	return true
}

// waited reports whether a Wait on group is reached for a start at node. A
// group the code cannot name has no Wait to find. A parameter is the caller's
// to wait on.
func (o *owners) waited(group *types.Var, node ast.Node) bool {
	if group == nil {
		return false
	}
	if o.index.IsParameter(group) {
		return true
	}
	use := o.index.DeclOf(node)
	for _, site := range o.waits[group] {
		if o.index.Reaches(group, ownership.Link{Actor: site.decl, Reader: use}) {
			return true
		}
	}
	return false
}
