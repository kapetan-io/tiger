package limitflow

import (
	"go/ast"
	"go/token"
	"go/types"
	"math/big"
	"slices"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/intrange"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
	"github.com/kapetan-io/tiger/internal/worklist"
)

// outcome is what tracing one value found: the places it enters with no
// bound, the constants too large to reach, the parameters it depends on, and
// the limit fields it reads straight into a loop.
type outcome struct {
	bad    []ast.Expr
	huge   []ast.Expr
	params []int
	fields []*types.Var
}

// bounded reports whether nothing in the outcome is a finding.
func (o outcome) bounded() bool {
	return len(o.bad) == 0 && len(o.huge) == 0
}

// nodeKind classifies a node of the tree a trace builds over one value.
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

// node is one step of the trace: a leaf, or a join of other nodes. A join
// of kind nodeAll is bounded when every child is; one of kind nodeAny, like
// min, when any child is.
type node struct {
	kind     nodeKind
	expr     ast.Expr
	param    int
	field    *types.Var
	children []int
	done     bool
	found    outcome
}

// task asks for one expression to be expanded into the node at index.
type task struct {
	index int
	expr  ast.Expr
	mode  directness
}

// tracer builds and evaluates the tree for one value. The tree is built with
// an index-advancing worklist and evaluated bottom-up, never recursively.
type tracer struct {
	s     *state
	fn    *function
	nodes []node
	work  []task
	vars  map[*types.Var]int
}

// joins maps the arithmetic operators to how their operands combine: all of
// them must be bounded, or either.
var joins = map[token.Token]nodeKind{
	token.ADD: nodeAll, token.SUB: nodeAll, token.MUL: nodeAll,
	token.SHL: nodeAll, token.OR: nodeAll, token.XOR: nodeAll,
	token.AND_NOT: nodeAll, token.AND: nodeAny,
}

// addsOrSubtracts are the operators of a running total.
var addsOrSubtracts = map[token.Token]bool{token.ADD: true, token.SUB: true}

// unaryJoins are the unary operators that leave a value as bounded as it was.
var unaryJoins = map[token.Token]bool{token.ADD: true, token.SUB: true, token.XOR: true}

// trace follows root back to where it enters the program.
func (s *state) trace(fn *function, root ast.Expr, mode directness) outcome {
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

// child adds a node under parent and queues expr to fill it.
func (t *tracer) child(parent int, expr ast.Expr, mode directness) {
	index := len(t.nodes)
	t.nodes = append(t.nodes, node{})
	t.nodes[parent].children = append(t.nodes[parent].children, index)
	t.work = append(t.work, task{index: index, expr: expr, mode: mode})
}

// leaf adds a finished node under parent.
func (t *tracer) leaf(parent int, kind nodeKind, expr ast.Expr) {
	index := len(t.nodes)
	t.nodes = append(t.nodes, node{kind: kind, expr: expr, done: false})
	t.nodes[parent].children = append(t.nodes[parent].children, index)
}

// set fills the node at index as a leaf.
func (t *tracer) set(index int, kind nodeKind, expr ast.Expr) {
	t.nodes[index].kind = kind
	t.nodes[index].expr = expr
}

// expand fills one node from its expression.
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

// constant fills the node when expr folds to a constant: bounded, or huge
// when the loop could never reach it.
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

// tooLarge reports whether a constant is at least half of its integer type's
// maximum, on a type wide enough that a loop to that count is out of reach.
func (t *tracer) tooLarge(known types.TypeAndValue) bool {
	span, ok := intrange.Of(known.Type, t.s.sizes())
	if !ok || span.High.Cmp(big.NewInt(1<<31-1)) < 0 {
		return false
	}
	value, exact := new(big.Int).SetString(known.Value.ExactString(), 10)
	return exact && value.Cmp(new(big.Int).Rsh(span.High, 1)) >= 0
}

// call fills the node from a conversion, a builtin, or a call to a declared
// function whose result is bounded when its arguments are.
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
			t.child(current.index, call.Args[index], indirect)
		}
		return
	}
	if current.mode == direct {
		t.set(current.index, nodeBounded, call)
		return
	}
	t.set(current.index, nodeUnbounded, call)
}

// builtin fills the node from len, cap, min or max.
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

// binary fills the node from arithmetic on two operands. A remainder is as
// bounded as its divisor, a quotient or right shift as bounded as its
// dividend, and a mask as bounded as either side.
func (t *tracer) binary(current task, expr *ast.BinaryExpr) {
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

// unary fills the node from a sign or complement of one operand.
func (t *tracer) unary(current task, expr *ast.UnaryExpr) {
	if !unaryJoins[expr.Op] {
		t.set(current.index, nodeUnbounded, expr)
		return
	}
	t.nodes[current.index].kind = nodeAll
	t.child(current.index, expr.X, current.mode)
}

// selector fills the node from a struct field read. A field known to bound a
// loop is another bounded limit. A field read straight into a loop header
// joins that set. A field of a struct with no name has nothing to record a
// fact against, so one read straight into a loop header is not judged. Any
// other field read is a value from outside.
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
	if current.mode == direct && key == "" {
		t.set(current.index, nodeBounded, expr)
		return
	}
	if current.mode == direct {
		t.nodes[current.index].field = field
		t.set(current.index, nodeField, expr)
		return
	}
	t.set(current.index, nodeUnbounded, expr)
}

// ident fills the node from a variable: a parameter passes the question to
// the caller, a local is as bounded as everything written into it, and a
// package-level variable is a value from outside.
func (t *tracer) ident(current task, ident *ast.Ident) {
	v, ok := t.s.pass.TypesInfo.ObjectOf(ident).(*types.Var)
	if !ok || v.IsField() {
		t.set(current.index, nodeUnbounded, ident)
		return
	}
	if index, isParam := t.fn.params[v]; isParam {
		t.nodes[current.index].param = index
		t.set(current.index, nodeParam, ident)
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

// local fills the node from every write into a local variable.
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

// written adds the node for one write into v.
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

// assigned adds the node for an assignment into v. A running total of
// bounded values is as bounded as what is added, because every loop in tiger's
// code ends. A variable that feeds itself any other way, such as doubling,
// grows without a bound, and a value that arrives with several results is not
// followed.
func (t *tracer) assigned(parent int, v *types.Var, stmt *ast.AssignStmt) {
	for i, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || t.s.pass.TypesInfo.ObjectOf(ident) != v {
			continue
		}
		if stmt.Tok == token.ADD_ASSIGN || stmt.Tok == token.SUB_ASSIGN {
			t.child(parent, stmt.Rhs[i], indirect)
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

// assignedValue adds the node for the value assigned into v. v = v + x adds
// x to a running total, and any other value that reads v feeds itself.
func (t *tracer) assignedValue(parent int, v *types.Var, value ast.Expr) {
	if !t.mentions(value, v) {
		t.child(parent, value, indirect)
		return
	}
	if t.clampsItself(value) {
		t.leaf(parent, nodeBounded, value)
		return
	}
	binary, ok := ast.Unparen(value).(*ast.BinaryExpr)
	if ok && addsOrSubtracts[binary.Op] && t.isVar(binary.X, v) {
		t.child(parent, binary.Y, indirect)
		return
	}
	t.leaf(parent, nodeUnbounded, value)
}

// clampsItself reports whether value is min(...) with an argument that is a
// constant or a length, as in spins = min(spins, spinsMax). The variable
// reads itself on purpose, to be cut down.
func (t *tracer) clampsItself(value ast.Expr) bool {
	call, ok := ast.Unparen(value).(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || ident.Name != "min" {
		return false
	}
	for _, arg := range call.Args {
		if t.declaredMaximum(arg) {
			return true
		}
	}
	return false
}

// isVar reports whether expr is a use of v.
func (t *tracer) isVar(expr ast.Expr, v *types.Var) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && t.s.pass.TypesInfo.ObjectOf(ident) == v
}

// declared adds the node for var v T or var v = value.
func (t *tracer) declared(parent int, v *types.Var, spec *ast.ValueSpec, use *ast.Ident) {
	if len(spec.Values) == 0 {
		t.leaf(parent, nodeBounded, use)
		return
	}
	for i, name := range spec.Names {
		if t.s.pass.TypesInfo.ObjectOf(name) == v && len(spec.Names) == len(spec.Values) {
			t.child(parent, spec.Values[i], indirect)
			return
		}
	}
	t.leaf(parent, nodeUnbounded, use)
}

// ranged adds the node for a range clause that assigns v. The index of a
// slice, array or string is below its length, the index of an integer range
// is below that integer, and anything else a range hands out is a value from
// outside.
func (t *tracer) ranged(parent int, v *types.Var, stmt *ast.RangeStmt) {
	key, isKey := stmt.Key.(*ast.Ident)
	if !isKey || t.s.pass.TypesInfo.ObjectOf(key) != v {
		if t.constantElements(stmt.X) {
			t.leaf(parent, nodeBounded, stmt.Value)
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
			t.child(parent, stmt.X, indirect)
			return
		}
		t.leaf(parent, nodeBounded, key)
	default:
		t.leaf(parent, nodeUnbounded, key)
	}
}

// constantElements reports whether expr is a literal slice or array whose
// elements are all constants.
func (t *tracer) constantElements(expr ast.Expr) bool {
	literal, ok := ast.Unparen(expr).(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, element := range literal.Elts {
		if t.s.pass.TypesInfo.Types[element].Value == nil {
			return false
		}
	}
	return true
}

// mentions reports whether expr reads v.
func (t *tracer) mentions(expr ast.Expr, v *types.Var) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if ok && t.s.pass.TypesInfo.ObjectOf(ident) == v {
			found = true
		}
		return !found
	})
	return found
}

// evaluate settles every node bottom-up. Children sit after their parents in
// creation order except a shared variable's node, so the pass repeats until
// the root is settled; a cycle never settles and counts as unbounded.
func (t *tracer) evaluate(root ast.Expr) outcome {
	rounds := len(t.nodes)
	for round := 0; round <= rounds; round++ {
		for index := len(t.nodes) - 1; index >= 0; index-- {
			t.settle(index)
		}
	}
	if !t.nodes[0].done {
		return outcome{bad: []ast.Expr{root}}
	}
	return t.nodes[0].found
}

// settle computes one node when all of its children are settled.
func (t *tracer) settle(index int) {
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

// joinAll merges the outcomes of children that must all be bounded.
func (t *tracer) joinAll(children []int) outcome {
	var merged outcome
	for _, child := range children {
		merged = merged.with(t.nodes[child].found)
	}
	return merged
}

// joinAny merges the outcomes of children of which one is enough. It takes
// the bounded child that needs the fewest parameters; when none is bounded,
// everything the children found.
func (t *tracer) joinAny(children []int) outcome {
	var best *outcome
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

// with combines two outcomes.
func (o outcome) with(other outcome) outcome {
	return outcome{
		bad:    append(slices.Clone(o.bad), other.bad...),
		huge:   append(slices.Clone(o.huge), other.huge...),
		params: append(slices.Clone(o.params), other.params...),
		fields: append(slices.Clone(o.fields), other.fields...),
	}
}
