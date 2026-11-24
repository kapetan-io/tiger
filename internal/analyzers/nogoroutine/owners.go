package nogoroutine

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/cfg"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/ownership"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/stopcase"
	"github.com/kapetan-io/tiger/internal/worklist"
)

var groupTypes = map[string]bool{
	"sync.WaitGroup":                   true,
	"golang.org/x/sync/errgroup.Group": true,
}

var startMethods = map[string]bool{"Go": true, "TryGo": true}

type owners struct {
	pass      *analysis.Pass
	index     *ownership.Index
	stops     *stopcase.Finder
	expired   []span
	graphs    map[*ast.BlockStmt]*cfg.CFG
	summaries map[*types.Var]waiters
	outside   map[*types.Var]bool
	walking   map[*ast.FuncLit]bool
}

type span struct {
	from, to token.Pos
}

func newOwners(pass *analysis.Pass) *owners {
	stops := stopcase.New(pass)
	o := &owners{
		pass: pass, index: stops.Index(), stops: stops,
		graphs:    map[*ast.BlockStmt]*cfg.CFG{},
		summaries: map[*types.Var]waiters{},
		outside:   map[*types.Var]bool{},
		walking:   map[*ast.FuncLit]bool{},
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

func (o *owners) noteExpired(clause *ast.CommClause) {
	if clause.Comm == nil || len(clause.Body) == 0 || !o.stops.ContextCase(clause.Comm) {
		return
	}
	o.expired = append(o.expired, span{from: clause.Body[0].Pos(), to: clause.End()})
}

func (o *owners) noteOutside(sel *ast.SelectorExpr) {
	if o.index.DeclOf(sel) != nil {
		return
	}
	if group, ok := o.groupMethod(sel, "Wait"); ok && group != nil {
		o.outside[group] = true
	}
}

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

const aliasHopsMax = 8

const _ = uint(worklist.NodesMax - aliasHopsMax)

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

func (o *owners) waited(group *types.Var, sel *ast.SelectorExpr) bool {
	if group == nil {
		return false
	}
	if o.index.IsParameter(group) {
		return true
	}
	decl := o.index.DeclOf(sel)
	if decl == nil {
		return o.outside[group]
	}
	always := o.summary(group)
	query := flowQuery{group: group, start: sel, anyPath: sel.Sel.Name == "TryGo"}
	for _, literal := range o.literalsAround(decl, group, sel) {
		if unwaited, exits := o.uncovered(literal, query, always); exits && !unwaited {
			return true
		}
	}
	if unwaited, _ := o.uncovered(functionOf(decl), query, always); !unwaited {
		return true
	}
	if !group.IsField() && group.Parent() != group.Pkg().Scope() {
		return false
	}
	for _, actor := range o.index.Decls() {
		if always[actor] && o.index.Reaches(group, ownership.Link{Actor: actor, Reader: decl}) {
			return true
		}
	}
	return false
}

func (o *owners) literalsAround(
	decl *ast.FuncDecl, group *types.Var, sel *ast.SelectorExpr,
) []function {
	var path []ast.Node
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		if node == nil || node.Pos() > sel.Pos() || sel.End() > node.End() {
			return false
		}
		path = append(path, node)
		return true
	})
	var around []function
	for i, node := range slices.Backward(path) {
		literal, ok := node.(*ast.FuncLit)
		if !ok {
			continue
		}
		var parent ast.Node
		for _, outer := range slices.Backward(path[:i]) {
			if !o.wraps(outer) {
				parent = outer
				break
			}
		}
		if o.isStartOn(parent, group, decl) {
			around = nil
			continue
		}
		around = append(around, function{decl: decl, typ: literal.Type, body: literal.Body})
	}
	return around
}

func (o *owners) wraps(node ast.Node) bool {
	switch outer := node.(type) {
	case *ast.ParenExpr:
		return true
	case *ast.CallExpr:
		return o.pass.TypesInfo.Types[outer.Fun].IsType()
	}
	return false
}

func (o *owners) isStartOn(node ast.Node, group *types.Var, decl *ast.FuncDecl) bool {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return false
	}
	start, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	started, ok := o.groupMethod(start, "")
	if !ok {
		return false
	}
	if started == nil {
		started = o.index.Object(addressed(start.X))
	}
	if started == nil && len(call.Args) > 0 {
		started = o.index.Object(addressed(call.Args[0]))
	}
	if started == nil || started == group {
		return started != nil
	}
	for _, source := range o.index.Definitions(started, decl) {
		if o.index.Object(addressed(source)) == group {
			return true
		}
	}
	return false
}

func addressed(expr ast.Expr) ast.Expr {
	expr = ast.Unparen(expr)
	if address, ok := expr.(*ast.UnaryExpr); ok && address.Op == token.AND {
		return address.X
	}
	return expr
}
