package variant

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/condition"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/intrange"
	"github.com/kapetan-io/tiger/internal/directive"
)

// place names where a measured value lives: a variable, or a field of one.
type place struct {
	root  *types.Var
	field *types.Var
}

// operand is one operand of a ranking: a constant, a variable or field, or
// the length of a variable or field. Exactly one of constant and length is
// set, or neither for a plain variable or field.
type operand struct {
	place
	number   int64
	constant bool
	length   bool
}

// atom prints the operand in the predicate language's syntax.
func (o operand) atom() directive.Atom {
	if o.constant {
		return directive.Atom{Kind: directive.AtomNumber, Int: o.number}
	}
	ref := o.root.Name()
	if o.field != nil {
		ref += "." + o.field.Name()
	}
	if o.length {
		return directive.Atom{Kind: directive.AtomLen, Ref: ref}
	}
	return directive.Atom{Kind: directive.AtomRef, Ref: ref}
}

// term is one signed operand of a ranking.
type term struct {
	operand
	minus bool
}

// ranking is a linear expression a loop must shrink.
type ranking []term

// variant prints the ranking as a //tiger:variant argument.
func (r ranking) variant() directive.Variant {
	var printed directive.Variant
	for _, t := range r {
		printed.Terms = append(printed.Terms, directive.VariantTerm{Minus: t.minus, Atom: t.atom()})
	}
	return printed
}

// lowHigh is a comparison normalized to low OP high, where OP is < or <=: a
// > or >= comparison is rewritten by swapping its operands.
type lowHigh struct {
	low  ast.Expr
	high ast.Expr
	op   token.Token
}

// normalize rewrites a comparison into low/high form, or reports the
// comparison is not one of the four ordering operators.
func normalize(binary *ast.BinaryExpr) (lowHigh, bool) {
	if binary.Op == token.LSS || binary.Op == token.LEQ {
		return lowHigh{low: binary.X, high: binary.Y, op: binary.Op}, true
	}
	mirrored, ok := condition.Mirror(binary.Op)
	if !ok {
		return lowHigh{}, false
	}
	if mirrored != token.LSS && mirrored != token.LEQ {
		return lowHigh{}, false
	}
	return lowHigh{low: binary.Y, high: binary.X, op: mirrored}, true
}

// candidate pairs a ranking with the comparison it was derived from.
type candidate struct {
	ranking ranking
	limits  lowHigh
}

// loopFacts is what a loop's condition and shape establish before its body
// is judged.
type loopFacts struct {
	*checker
	loop *ast.ForStmt
	// alwaysRuns holds the statements that run on every pass: the Post
	// clause and the body's direct statements.
	alwaysRuns map[ast.Node]bool
	// positive holds the variables the condition keeps above zero.
	positive map[place]bool
	// nonEmpty holds the slices the condition keeps from being empty.
	nonEmpty map[place]bool
}

// factsOf collects the guards every top-level && term of the condition
// establishes on each pass, and the statements that always run: the Post
// clause and the body's direct statements.
func (c *checker) factsOf(loop *ast.ForStmt) *loopFacts {
	found := &loopFacts{
		checker: c, loop: loop, alwaysRuns: map[ast.Node]bool{},
		positive: map[place]bool{}, nonEmpty: map[place]bool{},
	}
	for _, stmt := range loop.Body.List {
		found.alwaysRuns[stmt] = true
	}
	if loop.Post != nil {
		found.alwaysRuns[loop.Post] = true
	}
	for _, conjunct := range condition.Conjuncts(loop.Cond) {
		found.learn(conjunct)
	}
	return found
}

// learn records what one condition term guarantees: a variable above zero,
// or a slice that is not empty.
func (f *loopFacts) learn(conjunct ast.Expr) {
	binary, ok := conjunct.(*ast.BinaryExpr)
	if !ok {
		return
	}
	limits, ok := normalize(binary)
	if !ok {
		return
	}
	floor, ok := f.smallConstant(limits.low)
	if !ok {
		return
	}
	if floor < 0 {
		return
	}
	if floor == 0 && limits.op != token.LSS {
		return
	}
	measured, ok := f.operandOf(limits.high)
	if !ok || measured.constant {
		return
	}
	if measured.length {
		f.nonEmpty[measured.place] = true
		return
	}
	f.positive[measured.place] = true
}

// smallConstant returns expr's value when the compiler folded it to an
// integer that fits an int64.
func (c *checker) smallConstant(expr ast.Expr) (int64, bool) {
	return intrange.SmallConstant(c.pass.TypesInfo, expr)
}

// operandOf reads expr as a ranking operand.
func (c *checker) operandOf(expr ast.Expr) (operand, bool) {
	expr = ast.Unparen(expr)
	if number, ok := c.smallConstant(expr); ok {
		return operand{number: number, constant: true}, true
	}
	if call, ok := expr.(*ast.CallExpr); ok {
		return c.lengthOf(call)
	}
	where, ok := c.placeOf(expr)
	if !ok || !c.isInteger(expr) {
		return operand{}, false
	}
	return operand{place: where}, true
}

// lengthOf reads len(x) as a ranking operand when x is a slice, map or
// string.
func (c *checker) lengthOf(call *ast.CallExpr) (operand, bool) {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || len(call.Args) != 1 {
		return operand{}, false
	}
	builtin, ok := c.pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok || builtin.Name() != "len" {
		return operand{}, false
	}
	where, ok := c.placeOf(call.Args[0])
	if !ok || !lenable(c.pass.TypesInfo.TypeOf(call.Args[0])) {
		return operand{}, false
	}
	return operand{place: where, length: true}, true
}

// placeOf resolves expr to a variable or to a field of a variable.
func (c *checker) placeOf(expr ast.Expr) (place, bool) {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		v, ok := c.pass.TypesInfo.ObjectOf(e).(*types.Var)
		return place{root: v}, ok && !v.IsField()
	case *ast.SelectorExpr:
		selection := c.pass.TypesInfo.Selections[e]
		if selection == nil || selection.Kind() != types.FieldVal || len(selection.Index()) != 1 {
			return place{}, false
		}
		base, ok := ast.Unparen(e.X).(*ast.Ident)
		if !ok {
			return place{}, false
		}
		root, ok := c.pass.TypesInfo.ObjectOf(base).(*types.Var)
		field, isField := selection.Obj().(*types.Var)
		return place{root: root, field: field}, ok && isField
	}
	return place{}, false
}

// isInteger reports whether expr has an integer type.
func (c *checker) isInteger(expr ast.Expr) bool {
	found := c.pass.TypesInfo.TypeOf(expr)
	return found != nil && intrange.IsInteger(found)
}

// lenable reports whether t is a slice, map, or string, the len subjects the
// rules admit.
func lenable(t types.Type) bool {
	if t == nil {
		return false
	}
	switch under := t.Underlying().(type) {
	case *types.Slice, *types.Map:
		return true
	case *types.Basic:
		return under.Info()&types.IsString != 0
	}
	return false
}

// synthesize tries each top-level && term of the condition as the source of
// a ranking and returns the first one the body verifies.
func (c *checker) synthesize(loop *ast.ForStmt) (directive.Variant, bool) {
	facts := c.factsOf(loop)
	for _, conjunct := range condition.Conjuncts(loop.Cond) {
		found, ok := facts.measure(conjunct)
		if ok && facts.verify(found.ranking, found.limits).ok {
			return found.ranking.variant(), true
		}
	}
	return directive.Variant{}, false
}

// measure classifies one condition term against the closed
// recognized-comparison set, returning the ranking it supplies.
func (f *loopFacts) measure(conjunct ast.Expr) (candidate, bool) {
	binary, ok := conjunct.(*ast.BinaryExpr)
	if !ok {
		return candidate{}, false
	}
	limits, ok := normalize(binary)
	if !ok {
		return candidate{}, false
	}
	if found, ok := f.lengthMeasure(limits); ok {
		return candidate{ranking: found, limits: limits}, true
	}
	found, ok := f.linearMeasure(limits)
	return candidate{ranking: found, limits: limits}, ok
}

// lengthMeasure recognizes low OP len(s) with low a non-negative constant:
// the ranking is len(s) alone.
func (f *loopFacts) lengthMeasure(limits lowHigh) (ranking, bool) {
	call, ok := ast.Unparen(limits.high).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	length, ok := f.lengthOf(call)
	if !ok {
		return nil, false
	}
	floor, ok := f.smallConstant(limits.low)
	if !ok || floor < 0 {
		return nil, false
	}
	return ranking{{operand: length}}, true
}

// linearMeasure recognizes an integer comparison between two operands, each
// an integer variable, field, or constant, with at least one not constant.
// The ranking is high - low, dropping a zero low term.
func (f *loopFacts) linearMeasure(limits lowHigh) (ranking, bool) {
	low, ok := f.operandOf(limits.low)
	if !ok || low.length {
		return nil, false
	}
	high, ok := f.operandOf(limits.high)
	if !ok || high.length {
		return nil, false
	}
	if low.constant && high.constant {
		return nil, false
	}
	measure := ranking{{operand: high}}
	if !low.constant || low.number != 0 {
		measure = append(measure, term{operand: low, minus: true})
	}
	return measure, true
}

// verifyPinned resolves the pin's names against the loop and verifies the
// ranking against the top-level && terms of the condition. A term supplies
// the lower bound only when the pin is about the same variables the term
// compares, so a pin on n cannot ride on a condition about i and m.
func (c *checker) verifyPinned(loop *ast.ForStmt, pinned directive.Variant) verdict {
	facts := c.factsOf(loop)
	resolved, err := facts.resolve(pinned)
	if err != nil {
		return verdict{reason: err.Error()}
	}
	reason := "no top-level && term of the loop's condition is a recognized comparison"
	for _, conjunct := range condition.Conjuncts(loop.Cond) {
		found, ok := facts.measure(conjunct)
		if !ok {
			continue
		}
		if !found.ranking.covers(resolved) {
			reason = "the expression is about variables the loop's condition does not compare"
			continue
		}
		if !facts.accounts(found, resolved) {
			reason = "the expression leaves out a variable the loop changes"
			continue
		}
		return facts.verify(resolved, found.limits)
	}
	return verdict{reason: reason}
}

// accounts reports whether the pin names every variable the condition's
// ranking compares that the loop changes, with the same sign. A pin on n alone
// says nothing about i < n when the loop also moves i.
func (f *loopFacts) accounts(found candidate, pinned ranking) bool {
	for _, m := range found.ranking {
		if m.constant || f.movementOf(m.operand).trend == trendConstant {
			continue
		}
		named := false
		for _, t := range pinned {
			if !t.constant && t.place == m.place && t.length == m.length && t.minus == m.minus {
				named = true
			}
		}
		if !named {
			return false
		}
	}
	return true
}

// covers reports whether every variable or field the pinned ranking names is
// one this ranking names.
func (r ranking) covers(pinned ranking) bool {
	for _, t := range pinned {
		if t.constant {
			continue
		}
		known := false
		for _, m := range r {
			if !m.constant && m.place == t.place && m.length == t.length {
				known = true
			}
		}
		if !known {
			return false
		}
	}
	return true
}

// resolve binds the pin's names to the variables and fields they mean where
// the loop sits. It fails when a name binds to nothing the rules can follow.
func (f *loopFacts) resolve(pinned directive.Variant) (ranking, error) {
	var resolved ranking
	for _, pinTerm := range pinned.Terms {
		found, ok := f.resolveAtom(pinTerm.Atom)
		if !ok {
			return nil, errors.New(
				atomText(pinTerm.Atom) + " isn't a variable, field or length in scope")
		}
		resolved = append(resolved, term{operand: found, minus: pinTerm.Minus})
	}
	return resolved, nil
}

// resolveAtom binds one atom to an operand using the scope at the loop.
func (f *loopFacts) resolveAtom(atom directive.Atom) (operand, bool) {
	if atom.Kind == directive.AtomNumber {
		return operand{number: atom.Int, constant: true}, true
	}
	if atom.Kind != directive.AtomRef && atom.Kind != directive.AtomLen {
		return operand{}, false
	}
	names := strings.Split(atom.Ref, ".")
	pos := f.loop.Body.Pos()
	_, object := f.pass.Pkg.Scope().Innermost(pos).LookupParent(names[0], pos)
	root, ok := object.(*types.Var)
	if !ok || len(names) > 2 {
		return operand{}, false
	}
	found := operand{place: place{root: root}, length: atom.Kind == directive.AtomLen}
	held := root.Type()
	if len(names) == 2 {
		field, ok := fieldNamed(held, names[1], f.pass.Pkg)
		if !ok {
			return operand{}, false
		}
		found.field = field
		held = field.Type()
	}
	if found.length != lenable(held) {
		return operand{}, false
	}
	if found.length {
		return found, true
	}
	if !intrange.IsInteger(held) {
		return operand{}, false
	}
	return found, true
}

// fieldNamed finds the struct field called name on t or on the type t points
// to.
func fieldNamed(t types.Type, name string, pkg *types.Package) (*types.Var, bool) {
	object, _, _ := types.LookupFieldOrMethod(t, true, pkg, name)
	field, ok := object.(*types.Var)
	return field, ok && field.IsField()
}

// atomText prints one atom for use inside a verification-failure reason.
func atomText(a directive.Atom) string {
	switch a.Kind {
	case directive.AtomNumber:
		return strconv.FormatInt(a.Int, 10)
	case directive.AtomRef:
		return a.Ref
	case directive.AtomLen:
		return "len(" + a.Ref + ")"
	case directive.AtomNil:
		return "nil"
	default:
		assert.Unreachable("AtomKind: unhandled value")
	}
	return ""
}
