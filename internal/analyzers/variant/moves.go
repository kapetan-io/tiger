package variant

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/counter"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
)

// trend classifies how a measured value changes across one pass of a loop.
type trend int

const (
	trendUnknown trend = iota
	trendConstant
	trendIncreasing
	trendDecreasing
)

// movement is a trend and the constant step behind it.
type movement struct {
	trend trend
	size  int64
}

// verdict is the result of verifying a variant against a loop body.
type verdict struct {
	ok     bool
	reason string
}

// verify checks the ranking against the back-edge discipline: nothing can
// skip the move, every positive term is non-increasing, every negative term
// is non-decreasing, at least one term moves strictly, no term is unknown,
// and no mover wraps around the end of its type before reaching its limit.
func (f *loopFacts) verify(expression ranking, limits lowHigh) verdict {
	if f.skipsMove() {
		return verdict{reason: "a continue or goto can skip the loop's decrease"}
	}
	moved := false
	for _, t := range expression {
		step := f.movementOf(t.operand)
		name := atomText(t.atom())
		if step.trend == trendUnknown {
			return verdict{reason: name + " has no step tiger recognizes"}
		}
		if t.minus && step.trend == trendDecreasing {
			return verdict{
				reason: name + " decreases, so the expression grows instead of shrinking",
			}
		}
		if !t.minus && step.trend == trendIncreasing {
			return verdict{reason: name + " increases instead of decreasing"}
		}
		if step.trend == trendConstant {
			continue
		}
		moved = true
		if f.wraps(t.operand, step, limits) {
			return verdict{reason: name + " can wrap around the end of its type before it stops"}
		}
	}
	if !moved {
		return verdict{reason: "no term in the expression provably decreases"}
	}
	return verdict{ok: true}
}

// wraps reports whether the mover can step past the end of its integer type
// before it meets the other side of the condition. A mover that is not itself
// one side of that comparison is trusted, and so is a step of size 0 (a
// division or shift), which cannot overshoot. Whether the limit is a constant
// is left to counter.Reaches.
func (f *loopFacts) wraps(mover operand, step movement, limits lowHigh) bool {
	if mover.length || step.size == 0 {
		return false
	}
	counted := mover.root
	if mover.field != nil {
		counted = mover.field
	}
	march := counter.Term{
		Counter: counted, Inclusive: limits.op == token.LEQ, Size: step.size,
	}
	where, ok := f.placeOf(limits.low)
	switch {
	case step.trend == trendIncreasing && ok && where == mover.place:
		march.Limit = limits.high
	case step.trend == trendDecreasing:
		where, ok = f.placeOf(limits.high)
		if !ok || where != mover.place {
			return false
		}
		march.Limit, march.Down = limits.low, true
	default:
		return false
	}
	return !counter.Reaches(f.scope, march)
}

// movementOf classifies one ranking operand: constant when nothing writes it,
// strictly increasing or decreasing when its only write is an unconditional
// recognized move, unknown otherwise.
func (f *loopFacts) movementOf(measured operand) movement {
	if measured.constant {
		return movement{trend: trendConstant}
	}
	writes, held := f.writesOf(measured)
	if !held {
		return movement{}
	}
	if len(writes) == 0 {
		return movement{trend: trendConstant}
	}
	if len(writes) != 1 || !f.alwaysRuns[writes[0].Node] {
		return movement{}
	}
	if writes[0].Kind == mutation.Closure || writes[0].Kind == mutation.Range {
		return movement{}
	}
	if writes[0].Kind == mutation.Insert {
		return movement{}
	}
	return f.classify(writes[0].Node, measured)
}

// writesOf collects what writes the operand inside the loop. It reports
// false when something outside the loop body could write it too: a pointer
// or closure made before the loop, or a call that receives the struct that
// holds the field.
func (f *loopFacts) writesOf(measured operand) ([]mutation.Write, bool) {
	scope := f.scope
	if measured.field == nil {
		return f.collectIn(scope.Writes, measured.root), !scope.Escapes(measured.root)
	}
	if len(f.collectIn(scope.Writes, measured.root)) > 0 || scope.Escapes(measured.root) {
		return nil, false
	}
	if scope.FieldEscapes(measured.field) || scope.ReceivesOwner(f.loop, measured.root.Type()) {
		return nil, false
	}
	return f.collectIn(scope.FieldWrites, measured.field), true
}

// collectIn runs a collector over the loop's body and Post clause.
func (f *loopFacts) collectIn(
	collect func(ast.Node, *types.Var) []mutation.Write, v *types.Var,
) []mutation.Write {
	found := collect(f.loop.Body, v)
	if f.loop.Post != nil {
		found = append(found, collect(f.loop.Post, v)...)
	}
	return found
}

// classify reads one write as a move of the operand.
func (f *loopFacts) classify(node ast.Node, measured operand) movement {
	if measured.length {
		return f.classifyShrink(node, measured)
	}
	switch stmt := node.(type) {
	case *ast.IncDecStmt:
		if stmt.Tok == token.DEC {
			return f.movedBy(stmt.X, measured, movement{trend: trendDecreasing, size: 1})
		}
		return f.movedBy(stmt.X, measured, movement{trend: trendIncreasing, size: 1})
	case *ast.AssignStmt:
		return f.classifyAssign(stmt, measured)
	}
	return movement{}
}

// movedBy returns the movement when target is the operand and the step moves
// it by at least one.
func (f *loopFacts) movedBy(target ast.Expr, measured operand, moves movement) movement {
	if !f.isPlace(target, measured.place) || moves.size < 1 {
		return movement{}
	}
	return moves
}

// stepOps are the operators of a fixed step, and the way each moves its
// target.
var stepOps = map[token.Token]trend{token.ADD: trendIncreasing, token.SUB: trendDecreasing}

// shrinkOps are the operators that shrink a value above zero, and the
// smallest constant operand with which each makes progress.
var shrinkOps = map[token.Token]int64{token.QUO: 2, token.SHR: 1}

// compoundOps maps a compound assignment token to its operator.
var compoundOps = map[token.Token]token.Token{
	token.ADD_ASSIGN: token.ADD, token.SUB_ASSIGN: token.SUB,
	token.QUO_ASSIGN: token.QUO, token.SHR_ASSIGN: token.SHR,
}

// classifyAssign reads x += k, x -= k, x /= k, x >>= k and their x = x op k
// long forms. A division or shift only shrinks a value known to be above
// zero.
func (f *loopFacts) classifyAssign(stmt *ast.AssignStmt, measured operand) movement {
	if len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return movement{}
	}
	op, amountExpr := compoundOps[stmt.Tok], stmt.Rhs[0]
	if stmt.Tok == token.ASSIGN {
		binary, ok := ast.Unparen(stmt.Rhs[0]).(*ast.BinaryExpr)
		if !ok || !f.isPlace(binary.X, measured.place) {
			return movement{}
		}
		op, amountExpr = binary.Op, binary.Y
	}
	amount, ok := f.smallConstant(amountExpr)
	if !ok {
		return movement{}
	}
	if direction, isStep := stepOps[op]; isStep {
		return f.movedBy(stmt.Lhs[0], measured, movement{trend: direction, size: amount})
	}
	floor, isShrink := shrinkOps[op]
	if !isShrink || amount < floor || !f.positive[measured.place] {
		return movement{}
	}
	if !f.isPlace(stmt.Lhs[0], measured.place) {
		return movement{}
	}
	return movement{trend: trendDecreasing}
}

// isPlace reports whether expr names the given variable or field.
func (f *loopFacts) isPlace(expr ast.Expr, want place) bool {
	where, ok := f.placeOf(expr)
	return ok && where == want
}

// classifyShrink reads s = s[k:], s = s[:len(s)-k] and s = s[len(part):]
// with part shown not empty, the moves that shorten a slice or string.
func (f *loopFacts) classifyShrink(node ast.Node, measured operand) movement {
	stmt, ok := node.(*ast.AssignStmt)
	if !ok || stmt.Tok != token.ASSIGN || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return movement{}
	}
	slice, ok := ast.Unparen(stmt.Rhs[0]).(*ast.SliceExpr)
	if !ok || slice.Slice3 || !f.isPlace(stmt.Lhs[0], measured.place) ||
		!f.isPlace(slice.X, measured.place) {
		return movement{}
	}
	switch {
	case slice.Low != nil && slice.High == nil:
		if amount, ok := f.smallConstant(slice.Low); ok && amount >= 1 {
			return movement{trend: trendDecreasing}
		}
		if f.partNotEmpty(stmt, measured, slice.Low) {
			return movement{trend: trendDecreasing}
		}
	case slice.Low == nil && slice.High != nil:
		if f.trimsConstant(slice.High, measured) {
			return movement{trend: trendDecreasing}
		}
	}
	return movement{}
}

// trimsConstant reports whether high is len(s) - k with k a positive
// constant.
func (f *loopFacts) trimsConstant(high ast.Expr, measured operand) bool {
	binary, ok := ast.Unparen(high).(*ast.BinaryExpr)
	if !ok || binary.Op != token.SUB {
		return false
	}
	call, ok := ast.Unparen(binary.X).(*ast.CallExpr)
	if !ok {
		return false
	}
	length, ok := f.lengthOf(call)
	if !ok || length.place != measured.place {
		return false
	}
	amount, ok := f.smallConstant(binary.Y)
	return ok && amount >= 1
}

// partNotEmpty reports whether part is len(chunk) for a chunk that holds at
// least one element of measured when stmt runs. The condition guarantees measured is
// not empty; chunk starts as all of measured, or a constant-sized front of it,
// and is only ever cut to a constant-sized front before stmt.
func (f *loopFacts) partNotEmpty(stmt *ast.AssignStmt, measured operand, part ast.Expr) bool {
	if !f.nonEmpty[measured.place] {
		return false
	}
	call, ok := ast.Unparen(part).(*ast.CallExpr)
	if !ok {
		return false
	}
	length, ok := f.lengthOf(call)
	if !ok || length.field != nil || f.scope.Escapes(length.root) {
		return false
	}
	declared := false
	for _, write := range f.scope.Writes(f.loop.Body, length.root) {
		assign, ok := write.Node.(*ast.AssignStmt)
		if !ok || write.Kind != mutation.Assign || assign.End() > stmt.Pos() {
			return false
		}
		if assign.Tok == token.DEFINE {
			declared = f.alwaysRuns[assign] && f.startsAsPartOf(assign, measured)
			if !declared {
				return false
			}
			continue
		}
		if !f.cutsToFront(assign, length.root, measured.place) {
			return false
		}
	}
	return declared
}

// startsAsPartOf reports whether define is chunk := s or chunk := s[:k] with
// k a positive constant.
func (f *loopFacts) startsAsPartOf(define *ast.AssignStmt, measured operand) bool {
	if len(define.Lhs) != 1 || len(define.Rhs) != 1 {
		return false
	}
	if where, ok := f.placeOf(define.Rhs[0]); ok {
		return where == measured.place
	}
	return f.frontOf(define.Rhs[0], measured.place)
}

// cutsToFront reports whether assign is chunk = chunk[:k] or chunk =
// whole[:k] with k a positive constant, where whole is the slice the loop
// consumes.
func (f *loopFacts) cutsToFront(assign *ast.AssignStmt, chunk *types.Var, whole place) bool {
	if assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	if !f.isPlace(assign.Lhs[0], place{root: chunk}) {
		return false
	}
	return f.frontOf(assign.Rhs[0], place{root: chunk}) || f.frontOf(assign.Rhs[0], whole)
}

// frontOf reports whether expr is held[:k] with k a positive constant.
func (f *loopFacts) frontOf(expr ast.Expr, held place) bool {
	slice, ok := ast.Unparen(expr).(*ast.SliceExpr)
	if !ok || slice.Slice3 || slice.Low != nil || slice.High == nil {
		return false
	}
	if where, ok := f.placeOf(slice.X); !ok || where != held {
		return false
	}
	amount, ok := f.smallConstant(slice.High)
	return ok && amount >= 1
}

// skipsMove reports whether the body holds a continue that could skip the
// loop's move, or a goto. A continue inside a nested loop ends that loop's
// pass; a labeled continue to a label outside the body ends this one.
func (f *loopFacts) skipsMove() bool {
	var scopes []ast.Node
	ast.Inspect(f.loop.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
			scopes = append(scopes, n)
		}
		return true
	})
	found := false
	ast.Inspect(f.loop.Body, func(n ast.Node) bool {
		branch, ok := n.(*ast.BranchStmt)
		if !ok {
			return !found
		}
		switch {
		case branch.Tok == token.GOTO:
			found = true
		case branch.Tok != token.CONTINUE:
		case branch.Label != nil:
			label := f.pass.TypesInfo.ObjectOf(branch.Label)
			found = label == nil || label.Pos() < f.loop.Body.Pos()
		default:
			found = !insideAny(scopes, branch)
		}
		return !found
	})
	return found
}

// insideAny reports whether node sits inside one of the nested statements.
func insideAny(nested []ast.Node, node ast.Node) bool {
	for _, outer := range nested {
		if outer.Pos() <= node.Pos() && node.End() <= outer.End() {
			return true
		}
	}
	return false
}
