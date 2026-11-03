package limitflow

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
	"slices"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/intrange"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
	"github.com/kapetan-io/tiger/internal/worklist"
)

type traceResult struct {
	bad    []ast.Expr
	huge   []ast.Expr
	params []int
	fields []*types.Var
}

func (r traceResult) bounded() bool {
	return len(r.bad) == 0 && len(r.huge) == 0
}

type nodeKind int

const (
	nodeBounded nodeKind = iota
	nodeUnbounded
	nodeHuge
	nodeParam
	nodeField
	nodeAll
	nodeAny
)

type node struct {
	kind     nodeKind
	expr     ast.Expr
	param    int
	field    *types.Var
	children []int
	done     bool
	found    traceResult
}

type task struct {
	index int
	expr  ast.Expr
	mode  reach
}

type tracer struct {
	s     *state
	fn    *function
	nodes []node
	work  []task
	vars  map[*types.Var]int
}

var joins = map[token.Token]nodeKind{
	token.ADD: nodeAll, token.SUB: nodeAll, token.MUL: nodeAll,
	token.SHL: nodeAll, token.OR: nodeAll, token.XOR: nodeAll,
	token.AND_NOT: nodeAll, token.AND: nodeAny,
}

var addsOrSubtracts = map[token.Token]bool{token.ADD: true, token.SUB: true}

var unaryJoins = map[token.Token]bool{token.ADD: true}

func (s *state) trace(fn *function, root ast.Expr, mode reach) traceResult {
	t := &tracer{s: s, fn: fn, vars: map[*types.Var]int{}}
	t.nodes = []node{{}}
	t.work = []task{{index: 0, expr: root, mode: mode}}
	i := 0
	for ; i < len(t.work) && i < worklist.NodesMax; i++ {
		t.expand(t.work[i])
	}
	assert.Ok(i == len(t.work), "limitflow: trace reached worklist.NodesMax")
	return t.evaluate(root)
}

func (t *tracer) child(parent int, expr ast.Expr, mode reach) {
	index := len(t.nodes)
	t.nodes = append(t.nodes, node{})
	t.nodes[parent].children = append(t.nodes[parent].children, index)
	t.work = append(t.work, task{index: index, expr: expr, mode: mode})
}

func (t *tracer) leaf(parent int, kind nodeKind, expr ast.Expr) {
	index := len(t.nodes)
	t.nodes = append(t.nodes, node{kind: kind, expr: expr})
	t.nodes[parent].children = append(t.nodes[parent].children, index)
}

func (t *tracer) set(index int, kind nodeKind, expr ast.Expr) {
	t.nodes[index].kind = kind
	t.nodes[index].expr = expr
}

func (t *tracer) expand(current task) {
	expr := ast.Unparen(current.expr)
	if t.constant(current.index, expr) {
		return
	}
	if t.clamped(expr) {
		t.set(current.index, nodeBounded, expr)
		return
	}
	switch e := expr.(type) {
	case *ast.CallExpr:
		t.call(current, e)
	case *ast.BinaryExpr:
		t.binary(current, e)
	case *ast.UnaryExpr:
		t.unary(current, e)
	case *ast.Ident:
		t.ident(current, e)
	case *ast.SelectorExpr:
		t.selector(current, e)
	default:
		t.set(current.index, nodeUnbounded, expr)
	}
}

func (t *tracer) constant(index int, expr ast.Expr) bool {
	known := t.s.pass.TypesInfo.Types[expr]
	if known.Value == nil {
		return false
	}
	kind := nodeBounded
	if t.tooLarge(known) {
		kind = nodeHuge
	}
	t.set(index, kind, expr)
	return true
}

func (t *tracer) tooLarge(known types.TypeAndValue) bool {
	span, ok := intrange.Of(known.Type, t.fn.scope.Sizes())
	if !ok || !span.OutOfReach() {
		return false
	}
	value, exact := new(big.Int).SetString(known.Value.ExactString(), 10)
	return exact && value.Cmp(new(big.Int).Rsh(span.High, 1)) >= 0
}

func (t *tracer) call(current task, call *ast.CallExpr) {
	info := t.s.pass.TypesInfo
	if info.Types[call.Fun].IsType() && len(call.Args) == 1 {
		t.nodes[current.index].kind = nodeAll
		t.child(current.index, call.Args[0], current.mode)
		return
	}
	if ident, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if builtin, isBuiltin := info.Uses[ident].(*types.Builtin); isBuiltin {
			t.builtin(current, call, builtin.Name())
			return
		}
	}
	callee := t.s.callee(call)
	if callee == nil {
		t.set(current.index, nodeUnbounded, call)
		return
	}
	if depends, ok := t.s.resultOf(callee); ok {
		t.nodes[current.index].kind = nodeAll
		for _, index := range depends {
			if index >= len(call.Args) {
				t.leaf(current.index, nodeUnbounded, call)
				continue
			}
			t.child(current.index, call.Args[index], downstream)
		}
		return
	}
	if current.mode == atHeader {
		t.set(current.index, nodeBounded, call)
		return
	}
	t.set(current.index, nodeUnbounded, call)
}

func (t *tracer) builtin(current task, call *ast.CallExpr, name string) {
	switch name {
	case "len", "cap":
		t.set(current.index, nodeBounded, call)
	case "min":
		t.nodes[current.index].kind = nodeAny
		for _, arg := range call.Args {
			t.child(current.index, arg, current.mode)
		}
	case "max":
		t.nodes[current.index].kind = nodeAll
		for _, arg := range call.Args {
			t.child(current.index, arg, current.mode)
		}
	default:
		t.set(current.index, nodeUnbounded, call)
	}
}

func (t *tracer) binary(current task, expr *ast.BinaryExpr) {
	if !t.signSafe(expr) {
		t.set(current.index, nodeUnbounded, expr)
		return
	}
	if expr.Op == token.REM {
		t.nodes[current.index].kind = nodeAll
		t.child(current.index, expr.Y, current.mode)
		return
	}
	if expr.Op == token.QUO || expr.Op == token.SHR {
		t.nodes[current.index].kind = nodeAll
		t.child(current.index, expr.X, current.mode)
		return
	}
	kind, ok := joins[expr.Op]
	if !ok {
		t.set(current.index, nodeUnbounded, expr)
		return
	}
	t.nodes[current.index].kind = kind
	t.child(current.index, expr.X, current.mode)
	t.child(current.index, expr.Y, current.mode)
}

func (t *tracer) signSafe(expr *ast.BinaryExpr) bool {
	if expr.Op == token.SUB || expr.Op == token.REM {
		return t.nonNegative(expr.Y)
	}
	if expr.Op == token.QUO {
		return t.nonNegative(expr.X) || t.nonNegative(expr.Y)
	}
	if expr.Op == token.MUL || expr.Op == token.XOR {
		return t.nonNegative(expr.X) || t.nonNegative(expr.Y)
	}
	if expr.Op == token.AND_NOT {
		return t.nonNegative(expr.Y)
	}
	return true
}

func (t *tracer) nonNegative(expr ast.Expr) bool {
	known := t.s.pass.TypesInfo.Types[expr]
	if known.Value != nil {
		return constant.Sign(known.Value) >= 0
	}
	basic, ok := known.Type.Underlying().(*types.Basic)
	if ok && basic.Info()&types.IsUnsigned != 0 {
		return true
	}
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	return ok && (t.isBuiltin(call, "len") || t.isBuiltin(call, "cap"))
}

func (t *tracer) unary(current task, expr *ast.UnaryExpr) {
	if !unaryJoins[expr.Op] {
		t.set(current.index, nodeUnbounded, expr)
		return
	}
	t.nodes[current.index].kind = nodeAll
	t.child(current.index, expr.X, current.mode)
}

func (t *tracer) selector(current task, expr *ast.SelectorExpr) {
	selection := t.s.pass.TypesInfo.Selections[expr]
	if selection == nil || selection.Kind() != types.FieldVal {
		t.set(current.index, nodeUnbounded, expr)
		return
	}
	field, ok := selection.Obj().(*types.Var)
	if !ok {
		t.set(current.index, nodeUnbounded, expr)
		return
	}
	key := t.s.fieldKey(field)
	if key != "" && t.s.limitFields[key] {
		t.set(current.index, nodeBounded, expr)
		return
	}
	if current.mode == atHeader && key == "" {
		t.set(current.index, nodeBounded, expr)
		return
	}
	if current.mode == atHeader {
		t.nodes[current.index].field = field
		t.set(current.index, nodeField, expr)
		return
	}
	t.set(current.index, nodeUnbounded, expr)
}

func (t *tracer) ident(current task, ident *ast.Ident) {
	v, ok := t.s.pass.TypesInfo.ObjectOf(ident).(*types.Var)
	if !ok || v.IsField() {
		t.set(current.index, nodeUnbounded, ident)
		return
	}
	if index, isParam := t.fn.params[v]; isParam {
		t.parameter(current, v, index, ident)
		return
	}
	if v.Parent() == v.Pkg().Scope() {
		t.set(current.index, nodeUnbounded, ident)
		return
	}
	if earlier, seen := t.vars[v]; seen {
		t.nodes[current.index].kind = nodeAll
		t.nodes[current.index].children = []int{earlier}
		return
	}
	t.vars[v] = current.index
	t.local(current, v, ident)
}

func (t *tracer) parameter(current task, v *types.Var, index int, use *ast.Ident) {
	writes := t.fn.scope.Writes(t.fn.decl, v)
	if len(writes) == 0 {
		t.nodes[current.index].param = index
		t.set(current.index, nodeParam, use)
		return
	}
	if earlier, seen := t.vars[v]; seen {
		t.nodes[current.index].kind = nodeAll
		t.nodes[current.index].children = []int{earlier}
		return
	}
	t.vars[v] = current.index
	t.nodes[current.index].kind = nodeAll
	t.leaf(current.index, nodeParam, use)
	t.nodes[len(t.nodes)-1].param = index
	for _, write := range writes {
		t.written(current.index, v, write, use)
	}
}

func (t *tracer) local(current task, v *types.Var, use *ast.Ident) {
	t.nodes[current.index].kind = nodeAll
	if t.fn.scope.Escapes(v) {
		t.set(current.index, nodeUnbounded, use)
		return
	}
	writes := t.fn.scope.Writes(t.fn.decl, v)
	if len(writes) == 0 {
		kind := nodeUnbounded
		if t.fn.literalParams[v] {
			kind = nodeBounded
		}
		t.set(current.index, kind, use)
		return
	}
	for _, write := range writes {
		t.written(current.index, v, write, use)
	}
}

func (t *tracer) written(parent int, v *types.Var, write mutation.Write, use *ast.Ident) {
	switch stmt := write.Node.(type) {
	case *ast.AssignStmt:
		t.assigned(parent, v, stmt)
	case *ast.ValueSpec:
		t.declared(parent, v, stmt, use)
	case *ast.RangeStmt:
		t.ranged(parent, v, stmt)
	case *ast.IncDecStmt:
		t.leaf(parent, nodeBounded, stmt.X)
	default:
		t.leaf(parent, nodeUnbounded, use)
	}
}

func (t *tracer) assigned(parent int, v *types.Var, stmt *ast.AssignStmt) {
	for i, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || t.s.pass.TypesInfo.ObjectOf(ident) != v {
			continue
		}
		if stmt.Tok == token.ADD_ASSIGN || stmt.Tok == token.SUB_ASSIGN {
			t.child(parent, stmt.Rhs[i], downstream)
			return
		}
		plain := stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE
		if !plain || len(stmt.Lhs) != len(stmt.Rhs) {
			t.leaf(parent, nodeUnbounded, lhs)
			return
		}
		t.assignedValue(parent, v, stmt.Rhs[i])
		return
	}
}

func (t *tracer) assignedValue(parent int, v *types.Var, value ast.Expr) {
	if !t.fn.scope.Mentions(value, v) {
		t.child(parent, value, downstream)
		return
	}
	if t.clampsItself(value) {
		t.leaf(parent, nodeBounded, value)
		return
	}
	binary, ok := ast.Unparen(value).(*ast.BinaryExpr)
	if ok && addsOrSubtracts[binary.Op] && t.fn.scope.IsVar(binary.X, v) {
		t.child(parent, binary.Y, downstream)
		return
	}
	t.leaf(parent, nodeUnbounded, value)
}

func (t *tracer) clampsItself(value ast.Expr) bool {
	call, ok := ast.Unparen(value).(*ast.CallExpr)
	if !ok {
		return false
	}
	if !t.isBuiltin(call, "min") {
		return false
	}
	for _, arg := range call.Args {
		if t.declaredMaximum(arg) {
			return true
		}
	}
	return false
}

func (t *tracer) declared(parent int, v *types.Var, spec *ast.ValueSpec, use *ast.Ident) {
	if len(spec.Values) == 0 {
		t.leaf(parent, nodeBounded, use)
		return
	}
	for i, name := range spec.Names {
		if t.s.pass.TypesInfo.ObjectOf(name) == v && len(spec.Names) == len(spec.Values) {
			t.child(parent, spec.Values[i], downstream)
			return
		}
	}
	t.leaf(parent, nodeUnbounded, use)
}

func (t *tracer) ranged(parent int, v *types.Var, stmt *ast.RangeStmt) {
	key, isKey := stmt.Key.(*ast.Ident)
	if !isKey || t.s.pass.TypesInfo.ObjectOf(key) != v {
		literal, ok := ast.Unparen(stmt.X).(*ast.CompositeLit)
		if ok && t.constantElements(literal) {
			t.constantList(parent, literal)
			return
		}
		t.leaf(parent, nodeUnbounded, stmt.Value)
		return
	}
	switch under := t.s.pass.TypesInfo.TypeOf(stmt.X).Underlying().(type) {
	case *types.Slice, *types.Array:
		t.leaf(parent, nodeBounded, key)
	case *types.Basic:
		if under.Info()&types.IsInteger != 0 {
			t.child(parent, stmt.X, downstream)
			return
		}
		t.leaf(parent, nodeBounded, key)
	default:
		t.leaf(parent, nodeUnbounded, key)
	}
}

func (t *tracer) constantElements(literal *ast.CompositeLit) bool {
	for _, element := range literal.Elts {
		if t.s.pass.TypesInfo.Types[element].Value == nil {
			return false
		}
	}
	return true
}

func (t *tracer) constantList(parent int, literal *ast.CompositeLit) {
	for _, element := range literal.Elts {
		kind := nodeBounded
		if t.tooLarge(t.s.pass.TypesInfo.Types[element]) {
			kind = nodeHuge
		}
		t.leaf(parent, kind, element)
	}
}

func (t *tracer) evaluate(root ast.Expr) traceResult {
	rounds := len(t.nodes)
	for round := 0; round <= rounds; round++ {
		if t.nodes[0].done {
			continue
		}
		for index := len(t.nodes) - 1; index >= 0; index-- {
			t.resolveNode(index)
		}
	}
	if !t.nodes[0].done {
		return traceResult{bad: []ast.Expr{root}}
	}
	return t.nodes[0].found
}

func (t *tracer) resolveNode(index int) {
	current := &t.nodes[index]
	if current.done {
		return
	}
	for _, child := range current.children {
		if !t.nodes[child].done {
			return
		}
	}
	current.done = true
	switch current.kind {
	case nodeBounded:
	case nodeUnbounded:
		current.found.bad = []ast.Expr{current.expr}
	case nodeHuge:
		current.found.huge = []ast.Expr{current.expr}
	case nodeParam:
		current.found.params = []int{current.param}
	case nodeField:
		current.found.fields = []*types.Var{current.field}
	case nodeAll:
		current.found = t.joinAll(current.children)
	case nodeAny:
		current.found = t.joinAny(current.children)
	default:
		assert.Unreachable("nodeKind: unhandled value")
	}
}

func (t *tracer) joinAll(children []int) traceResult {
	var merged traceResult
	for _, child := range children {
		merged = merged.merge(t.nodes[child].found)
	}
	return merged
}

func (t *tracer) joinAny(children []int) traceResult {
	var best *traceResult
	for _, child := range children {
		found := t.nodes[child].found
		if !found.bounded() {
			continue
		}
		if best == nil || len(found.params) < len(best.params) {
			best = &found
		}
	}
	if best != nil {
		return *best
	}
	return t.joinAll(children)
}

func (r traceResult) merge(other traceResult) traceResult {
	return traceResult{
		bad:    append(slices.Clone(r.bad), other.bad...),
		huge:   append(slices.Clone(r.huge), other.huge...),
		params: append(slices.Clone(r.params), other.params...),
		fields: append(slices.Clone(r.fields), other.fields...),
	}
}
