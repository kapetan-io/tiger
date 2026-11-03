// Package boundedloop enforces TS-S02, TS-S03 and TS-S23: every loop has an
// upper bound, an unbounded event loop selects on a stop case, and the stop
// case is the loop's only exit.
//
// A RangeStmt over a slice, array, map, string, integer, or a standard
// library iterator is bounded by its own syntax. A RangeStmt over a channel
// or any other iterator is not: termination depends on code the analyzer
// cannot see, so it is a TS-S02 finding. A ForStmt with no Cond ("for {}")
// is either the TS-S03 event-loop shape — a direct SelectStmt with a stop
// case (see internal/analyzers/internal/stopcase) — or a TS-S02 finding.
//
// A ForStmt with a Cond is bounded when its Post advances a sound counter
// (see internal/analyzers/internal/counter), when TS-V01 verified a variant
// for it, when it runs until a context parameter is cancelled
// (ctx.Err() == nil), or when Cond — walking through parens, !,
// && (either operand bounded), and || (every operand bounded) — reduces to
// one of: a comparison against a constant, a len(), or a cap(); a
// monotone-counter comparison against zero (x != 0, with every assignment
// to x inside the loop — matched by object identity, so a shadow is a
// different variable — provably a right-shift or a division by a constant
// greater than one, and at least one of them guaranteed every iteration:
// the Post clause, or a direct body statement no continue or goto can
// skip); or a tuple-post reversal (i < j / i <= j, with the Post moving i
// up and j down by the same constant step, closing the gap).
// A FuncLit is a frame boundary for the monotone-counter and tuple-post
// direction scans: an assignment inside a closure counts neither for nor
// against the proof, matching deferdistance's treatment of the same
// shape.
//
// A counter whose limit stands in for a proof is a safety cap, and the loop
// must report when the cap is hit (see cap.go).
//
// Any other Cond is a TS-S02 finding, a store cursor loop (rows.Next())
// included. //tiger:batched waives TS-M10 only; a cursor loop states its
// limit in the header (ADR-0014).
package boundedloop

import (
	"errors"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/cancel"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/counter"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/intrange"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/stopcase"
	"github.com/kapetan-io/tiger/internal/analyzers/variant"
	"github.com/kapetan-io/tiger/internal/worklist"
)

const (
	msgUnbounded = "TS-S02: tiger can't tell how many times this loop runs: its condition is " +
		"not a constant, a len, or a counter — add a cap that fails when the cap is hit, " +
		"or make it an event loop that selects on ctx.Done()"
	msgNoSelect = "TS-S02: this for loop has no condition and no select on ctx.Done(), so " +
		"nothing bounds it — add a cap (for tries := 0; tries < max; tries++) that fails when " +
		"the cap is hit, or select on ctx.Done() inside it"
	msgChannelRange = "TS-S02: this loop ranges over a channel, so it ends only when some " +
		"other goroutine closes the channel — add a counter cap that fails when the cap is " +
		"hit, or make it an event loop that selects on ctx.Done()"
	msgCounterUnsound = "TS-S02: this counter does not bound the loop: it must count toward its " +
		"limit in a top-level && term and arrive before wrapping around its type, and neither " +
		"may change in the body — add a cap that fails when hit"
	msgIterator = "TS-S02: this loop ranges over an iterator tiger can't see into, so nothing " +
		"says it ends — range over a slice, a map, or a standard library iterator, or add a " +
		"counter cap that fails when the cap is hit"
	msgSelectNoDone = "TS-S03: this event loop's select has no case that stops the loop — add " +
		"case <-ctx.Done(): return or a case on a channel the package closes"
	msgExitsOutside = "TS-S23: this %s leaves the event loop without its stop case, so cleanup " +
		"after the stop never runs — make the stop case the loop's only exit, as " +
		"net/http's Server.Shutdown does"
)

var Analyzer = &analysis.Analyzer{
	Name:     "boundedloop",
	Doc:      "TS-S02 and TS-S03: every loop has an upper bound or the event-loop shape.",
	Requires: []*analysis.Analyzer{variant.Analyzer},
	Run:      run,
}

type checker struct {
	pass      *analysis.Pass
	scope     mutation.Scope
	verified  variant.Verified
	following map[ast.Stmt][]ast.Stmt
	params    map[*types.Var]bool
	stops     *stopcase.Finder
}

func run(pass *analysis.Pass) (any, error) {
	verified, ok := pass.ResultOf[variant.Analyzer].(variant.Verified)
	if !ok {
		return nil, errors.New("boundedloop: the variant result is missing")
	}
	stops := stopcase.New(pass)
	for _, scope := range mutation.Declarations(pass) {
		decl := scope.Root()
		check := &checker{
			pass: pass, scope: scope, verified: verified,
			following: followingStatements(decl), params: scope.Parameters(), stops: stops,
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			switch loop := node.(type) {
			case *ast.RangeStmt:
				check.checkRange(loop)
			case *ast.ForStmt:
				check.checkFor(loop)
			}
			return true
		})
	}
	return nil, nil
}

func followingStatements(decl ast.Node) map[ast.Stmt][]ast.Stmt {
	following := map[ast.Stmt][]ast.Stmt{}
	ast.Inspect(decl, func(node ast.Node) bool {
		var list []ast.Stmt
		switch block := node.(type) {
		case *ast.BlockStmt:
			list = block.List
		case *ast.CaseClause:
			list = block.Body
		case *ast.CommClause:
			list = block.Body
		}
		for i, stmt := range list {
			following[stmt] = list[i+1:]
			if labeled, ok := stmt.(*ast.LabeledStmt); ok {
				following[labeled.Stmt] = list[i+1:]
			}
		}
		return true
	})
	return following
}

func (c *checker) checkRange(loop *ast.RangeStmt) {
	rangeType := c.pass.TypesInfo.TypeOf(loop.X)
	if rangeType == nil {
		return
	}
	switch rangeType.Underlying().(type) {
	case *types.Chan:
		c.pass.Report(analysis.Diagnostic{
			Pos:      loop.Pos(),
			Category: "TS-S02",
			Message:  msgChannelRange,
		})
	case *types.Signature:
		if !c.standardIterator(loop.X) {
			c.pass.Report(analysis.Diagnostic{
				Pos:      loop.Pos(),
				Category: "TS-S02",
				Message:  msgIterator,
			})
		}
	}
}

func (c *checker) standardIterator(expr ast.Expr) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return false
	}
	var name *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		name = fun
	case *ast.SelectorExpr:
		name = fun.Sel
	default:
		return false
	}
	fn, ok := c.pass.TypesInfo.Uses[name].(*types.Func)
	if !ok || fn.Pkg() == nil || !c.scope.External(fn.Pkg()) {
		return false
	}
	first, _, _ := strings.Cut(fn.Pkg().Path(), "/")
	return !strings.Contains(first, ".")
}

func (c *checker) checkFor(loop *ast.ForStmt) {
	if loop.Cond == nil {
		c.checkEventLoop(loop)
		return
	}
	terms := counter.Terms(c.scope, loop)
	if len(terms) > 0 {
		c.checkCap(loop, terms)
		return
	}
	if c.bounded(loop) {
		return
	}
	message := msgUnbounded
	if counter.Counted(c.scope, loop) {
		message = msgCounterUnsound
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:      loop.Cond.Pos(),
		Category: "TS-S02",
		Message:  message,
	})
}

func (c *checker) bounded(loop *ast.ForStmt) bool {
	if c.verified[loop] {
		return true
	}
	if cancel.UntilCancelled(c.pass.TypesInfo, c.params, loop.Cond) {
		return true
	}
	if counter.Counted(c.scope, loop) {
		return false
	}
	return condBounded(c.pass, loop, loop.Cond, plainContext)
}

func (c *checker) checkEventLoop(loop *ast.ForStmt) {
	sel := directSelect(loop.Body)
	if sel == nil {
		c.pass.Report(analysis.Diagnostic{
			Pos:      loop.Pos(),
			Category: "TS-S02",
			Message:  msgNoSelect,
		})
		return
	}
	if selectHasDefault(sel) {
		return
	}
	if !c.selectHasStopCase(loop, sel) {
		c.pass.Report(analysis.Diagnostic{
			Pos:      sel.Pos(),
			Category: "TS-S03",
			Message:  msgSelectNoDone,
		})
		return
	}
	c.checkExits(loop)
}

func selectHasDefault(sel *ast.SelectStmt) bool {
	for _, clause := range sel.Body.List {
		if comm, ok := clause.(*ast.CommClause); ok && comm.Comm == nil {
			return true
		}
	}
	return false
}

func directSelect(block *ast.BlockStmt) *ast.SelectStmt {
	for _, stmt := range block.List {
		if sel, ok := stmt.(*ast.SelectStmt); ok {
			return sel
		}
	}
	return nil
}

func (c *checker) selectHasStopCase(loop *ast.ForStmt, sel *ast.SelectStmt) bool {
	for _, clause := range sel.Body.List {
		comm, ok := clause.(*ast.CommClause)
		if ok && c.stops.Case(comm.Comm) && c.leavesLoop(loop, comm) {
			return true
		}
	}
	return false
}

type polarity bool

const (
	plainContext   polarity = false
	negatedContext polarity = true
)

type counterSign bool

const (
	signedCounter   counterSign = false
	unsignedCounter counterSign = true
)

type condOp int

const (
	condLeaf condOp = iota
	condAll
	condAny
)

type condNode struct {
	op       condOp
	verdict  bool
	children []*condNode
}

type condTask struct {
	expr ast.Expr
	p    polarity
	node *condNode
}

func condBounded(pass *analysis.Pass, loop *ast.ForStmt, expr ast.Expr, p polarity) bool {
	root := &condNode{}
	nodes := []*condNode{root}
	work := []condTask{{expr: expr, p: p, node: root}}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		task := work[i]
		switch e := task.expr.(type) {
		case *ast.ParenExpr:
			task.node.op = condAll
			work = append(work, condTask{expr: e.X, p: task.p, node: condChild(task.node, &nodes)})
		case *ast.UnaryExpr:
			if e.Op != token.NOT {
				continue
			}
			task.node.op = condAll
			work = append(work, condTask{expr: e.X, p: !task.p, node: condChild(task.node, &nodes)})
		case *ast.BinaryExpr:
			if e.Op != token.LAND && e.Op != token.LOR {
				task.node.verdict = leafBounded(pass, loop, e, task.p)
				continue
			}
			task.node.op = condAny
			if (e.Op == token.LAND) == (task.p == negatedContext) {
				task.node.op = condAll
			}
			work = append(work,
				condTask{expr: e.X, p: task.p, node: condChild(task.node, &nodes)},
				condTask{expr: e.Y, p: task.p, node: condChild(task.node, &nodes)})
		}
	}
	assert.Ok(i == len(work), "condBounded: worklist reached worklist.NodesMax")
	for i := len(nodes) - 1; i >= 0; i-- {
		evalCond(nodes[i])
	}
	return root.verdict
}

func condChild(parent *condNode, nodes *[]*condNode) *condNode {
	created := &condNode{}
	parent.children = append(parent.children, created)
	*nodes = append(*nodes, created)
	return created
}

func evalCond(node *condNode) {
	switch node.op {
	case condLeaf:
	case condAll:
		node.verdict = true
		for _, held := range node.children {
			if !held.verdict {
				node.verdict = false
			}
		}
	case condAny:
		for _, held := range node.children {
			if held.verdict {
				node.verdict = true
			}
		}
	default:
		assert.Unreachable("condOp: unhandled value")
	}
}

func leafBounded(
	pass *analysis.Pass,
	loop *ast.ForStmt,
	binary *ast.BinaryExpr,
	p polarity,
) bool {
	comparison := binary.Op == token.LSS || binary.Op == token.LEQ ||
		binary.Op == token.GTR || binary.Op == token.GEQ
	boundOperand := isBoundOperand(pass, binary.X) || isBoundOperand(pass, binary.Y)
	if comparison && boundOperand {
		return true
	}
	if p == negatedContext {
		return false
	}
	if binary.Op == token.NEQ {
		return monotoneToZero(pass, loop, binary)
	}
	if binary.Op == token.LSS || binary.Op == token.LEQ {
		return tuplePostReversal(pass, loop, binary)
	}
	return false
}

func monotoneToZero(pass *analysis.Pass, loop *ast.ForStmt, binary *ast.BinaryExpr) bool {
	ident, ok := neqZeroIdent(pass, binary)
	if !ok {
		return false
	}
	target, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	if !ok {
		return false
	}
	basic, ok := target.Type().Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsInteger == 0 {
		return false
	}
	sign := signedCounter
	if basic.Info()&types.IsUnsigned != 0 {
		sign = unsignedCounter
	}
	assignments := loopAssignments(pass, target, loop)
	if len(assignments) == 0 {
		return false
	}
	for _, node := range assignments {
		if !zeroMovingAssignment(pass, node, sign) {
			return false
		}
	}
	return unconditionalAssignment(assignments, loop)
}

func unconditionalAssignment(assignments []ast.Node, loop *ast.ForStmt) bool {
	for _, node := range assignments {
		if loop.Post == node {
			return true
		}
	}
	for i, stmt := range loop.Body.List {
		for _, node := range assignments {
			if stmt == node {
				return !skipsBefore(loop.Body.List[:i])
			}
		}
	}
	return false
}

func skipsBefore(stmts []ast.Stmt) bool {
	for _, stmt := range stmts {
		skip := false
		ast.Inspect(stmt, func(n ast.Node) bool {
			if _, isFuncLit := n.(*ast.FuncLit); isFuncLit {
				return false
			}
			if branch, ok := n.(*ast.BranchStmt); ok {
				if branch.Tok == token.CONTINUE || branch.Tok == token.GOTO {
					skip = true
				}
			}
			return true
		})
		if skip {
			return true
		}
	}
	return false
}

func neqZeroIdent(pass *analysis.Pass, binary *ast.BinaryExpr) (*ast.Ident, bool) {
	if binary.Op != token.NEQ {
		return nil, false
	}
	if ident, ok := binary.X.(*ast.Ident); ok && isZero(pass, binary.Y) {
		return ident, true
	}
	if ident, ok := binary.Y.(*ast.Ident); ok && isZero(pass, binary.X) {
		return ident, true
	}
	return nil, false
}

func isZero(pass *analysis.Pass, expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.INT && e.Value == "0"
	case *ast.Ident:
		c, ok := pass.TypesInfo.Uses[e].(*types.Const)
		if !ok {
			return false
		}
		return constant.Sign(c.Val()) == 0
	}
	return false
}

func zeroMovingAssignment(pass *analysis.Pass, node ast.Node, sign counterSign) bool {
	assign, ok := node.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	if assign.Tok == token.SHR_ASSIGN {
		return sign == unsignedCounter && isConstAtLeast(pass, assign.Rhs[0], 1)
	}
	if assign.Tok == token.QUO_ASSIGN {
		return isConstAtLeast(pass, assign.Rhs[0], 2)
	}
	if assign.Tok == token.ASSIGN {
		return selfShiftOrDivide(pass, assign, sign)
	}
	return false
}

func selfShiftOrDivide(pass *analysis.Pass, assign *ast.AssignStmt, sign counterSign) bool {
	lhsIdent, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return false
	}
	binary, ok := assign.Rhs[0].(*ast.BinaryExpr)
	if !ok {
		return false
	}
	rhsIdent, ok := binary.X.(*ast.Ident)
	if !ok || rhsIdent.Name != lhsIdent.Name {
		return false
	}
	if binary.Op == token.SHR {
		return sign == unsignedCounter && isConstAtLeast(pass, binary.Y, 1)
	}
	if binary.Op == token.QUO {
		return isConstAtLeast(pass, binary.Y, 2)
	}
	return false
}

func isConstAtLeast(pass *analysis.Pass, expr ast.Expr, min int64) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.INT {
			return false
		}
		n, err := strconv.ParseInt(e.Value, 0, 64)
		return err == nil && n >= min
	case *ast.Ident:
		c, ok := pass.TypesInfo.Uses[e].(*types.Const)
		if !ok {
			return false
		}
		return constant.Compare(c.Val(), token.GEQ, constant.MakeInt64(min))
	default:
		return false
	}
}

func tuplePostReversal(pass *analysis.Pass, loop *ast.ForStmt, binary *ast.BinaryExpr) bool {
	loIdent, ok := binary.X.(*ast.Ident)
	if !ok || !intrange.IsInteger(pass.TypesInfo.TypeOf(loIdent)) {
		return false
	}
	hiIdent, ok := binary.Y.(*ast.Ident)
	if !ok || !intrange.IsInteger(pass.TypesInfo.TypeOf(hiIdent)) {
		return false
	}
	assign, ok := loop.Post.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != len(assign.Rhs) {
		return false
	}
	if !postShrinksGap(assign, gapNames{lo: loIdent.Name, hi: hiIdent.Name}) {
		return false
	}
	loVar, ok := pass.TypesInfo.Uses[loIdent].(*types.Var)
	if !ok {
		return false
	}
	hiVar, ok := pass.TypesInfo.Uses[hiIdent].(*types.Var)
	if !ok {
		return false
	}
	if len(assignmentsTo(pass, loVar, loop.Body)) > 0 {
		return false
	}
	if len(assignmentsTo(pass, hiVar, loop.Body)) > 0 {
		return false
	}
	return true
}

type gapNames struct {
	lo, hi string
}

func postShrinksGap(assign *ast.AssignStmt, names gapNames) bool {
	loMoved, hiMoved := false, false
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		switch ident.Name {
		case names.lo:
			loMoved = stepDirection(names.lo, assign.Rhs[i]) > 0
		case names.hi:
			hiMoved = stepDirection(names.hi, assign.Rhs[i]) < 0
		}
	}
	return loMoved && hiMoved
}

func stepDirection(name string, rhs ast.Expr) int {
	binary, ok := rhs.(*ast.BinaryExpr)
	if !ok {
		return 0
	}
	ident, ok := binary.X.(*ast.Ident)
	if !ok || ident.Name != name {
		return 0
	}
	lit, ok := binary.Y.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0
	}
	n, err := strconv.ParseInt(lit.Value, 0, 64)
	if err != nil || n <= 0 {
		return 0
	}
	if binary.Op == token.ADD {
		return 1
	}
	if binary.Op == token.SUB {
		return -1
	}
	return 0
}

func loopAssignments(pass *analysis.Pass, target *types.Var, loop *ast.ForStmt) []ast.Node {
	var found []ast.Node
	if loop.Post != nil {
		found = append(found, assignmentsTo(pass, target, loop.Post)...)
	}
	found = append(found, assignmentsTo(pass, target, loop.Body)...)
	return found
}

func assignmentsTo(pass *analysis.Pass, target *types.Var, node ast.Node) []ast.Node {
	uses := func(expr ast.Expr) bool {
		ident, ok := expr.(*ast.Ident)
		return ok && pass.TypesInfo.Uses[ident] == target
	}
	var found []ast.Node
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if _, isFuncLit := n.(*ast.FuncLit); isFuncLit {
			return false
		}
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				if uses(lhs) {
					found = append(found, stmt)
				}
			}
		case *ast.IncDecStmt:
			if uses(stmt.X) {
				found = append(found, stmt)
			}
		case *ast.RangeStmt:
			if uses(stmt.Key) || uses(stmt.Value) {
				found = append(found, stmt)
			}
		}
		return true
	})
	return found
}

func isBoundOperand(pass *analysis.Pass, expr ast.Expr) bool {
	for range 64 {
		wrapped, ok := expr.(*ast.ParenExpr)
		if !ok {
			break
		}
		expr = wrapped.X
	}
	switch operand := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		_, isConst := pass.TypesInfo.Uses[operand].(*types.Const)
		return isConst
	case *ast.SelectorExpr:
		_, isConst := pass.TypesInfo.Uses[operand.Sel].(*types.Const)
		return isConst
	case *ast.CallExpr:
		return isLenOrCap(pass, operand)
	default:
		return false
	}
}

func isLenOrCap(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return false
	}
	return builtin.Name() == "len" || builtin.Name() == "cap"
}
