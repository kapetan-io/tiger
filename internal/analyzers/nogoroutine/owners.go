package nogoroutine

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/cfg"

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
	pass      *analysis.Pass
	index     *ownership.Index
	stops     *stopcase.Finder
	expired   []span
	graphs    map[*ast.FuncDecl]*cfg.CFG
	summaries map[*types.Var]waiters
	outside   map[*types.Var]bool
}

// span is a range of source.
type span struct {
	from, to token.Pos
}

// newOwners finds the select cases that mean a caller gave up waiting, and the
// groups waited on outside any function.
func newOwners(pass *analysis.Pass) *owners {
	stops := stopcase.New(pass)
	o := &owners{
		pass: pass, index: stops.Index(), stops: stops,
		graphs:    map[*ast.FuncDecl]*cfg.CFG{},
		summaries: map[*types.Var]waiters{},
		outside:   map[*types.Var]bool{},
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch found := node.(type) {
			case *ast.CommClause:
				o.noteExpired(found)
			case *ast.SelectorExpr:
				o.noteOutside(found)
			}
			return true
		})
	}
	return o
}

// noteExpired records the body of a select case that receives from ctx.Done():
// the early return of a function whose caller gave up waiting.
func (o *owners) noteExpired(clause *ast.CommClause) {
	if clause.Comm == nil || len(clause.Body) == 0 || !o.stops.ContextCase(clause.Comm) {
		return
	}
	o.expired = append(o.expired, span{from: clause.Body[0].Pos(), to: clause.End()})
}

// noteOutside records a Wait on a group made outside any function, such as in
// a package variable's initializer.
func (o *owners) noteOutside(sel *ast.SelectorExpr) {
	if o.index.DeclOf(sel) != nil {
		return
	}
	if group, ok := o.groupMethod(sel, "Wait"); ok && group != nil {
		o.outside[group] = true
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

// waited reports whether a Wait on group is reached on every success path
// after a start at sel. A group the code cannot name has no Wait to find. A
// parameter is the caller's to wait on. A group in a field or a package
// variable may instead be waited by an exported method that always waits.
func (o *owners) waited(group *types.Var, sel *ast.SelectorExpr) bool {
	if group == nil {
		return false
	}
	if o.index.IsParameter(group) {
		return true
	}
	start := o.index.DeclOf(sel)
	if start == nil {
		return o.outside[group]
	}
	always := o.summary(group)
	query := flowQuery{group: group, start: sel, anyPath: sel.Sel.Name == "TryGo"}
	if !o.uncovered(start, query, always) {
		return true
	}
	if !group.IsField() && group.Parent() != group.Pkg().Scope() {
		return false
	}
	for _, decl := range o.index.Decls() {
		if always[decl] && o.index.Reaches(group, ownership.Link{Actor: decl, Reader: start}) {
			return true
		}
	}
	return false
}
