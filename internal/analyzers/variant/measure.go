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

type place struct {
	root  *types.Var
	field *types.Var
}

type operand struct {
	place
	number   int64
	constant bool
	length   bool
}

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

type term struct {
	operand
	minus bool
}

type ranking []term

func (r ranking) variant() directive.Variant {
	var printed directive.Variant
	for _, t := range r {
		printed.Terms = append(printed.Terms, directive.VariantTerm{Minus: t.minus, Atom: t.atom()})
	}
	return printed
}

type lowHigh struct {
	low  ast.Expr
	high ast.Expr
	op   token.Token
}

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

type candidate struct {
	ranking ranking
	limits  lowHigh
}

type loopFacts struct {
	*checker
	loop       *ast.ForStmt
	alwaysRuns map[ast.Node]bool
	positive   map[place]bool
	nonEmpty   map[place]bool
}

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

func (c *checker) smallConstant(expr ast.Expr) (int64, bool) {
	return intrange.SmallConstant(c.pass.TypesInfo, expr)
}

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

func (c *checker) isInteger(expr ast.Expr) bool {
	found := c.pass.TypesInfo.TypeOf(expr)
	return found != nil && intrange.IsInteger(found)
}

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

func fieldNamed(t types.Type, name string, pkg *types.Package) (*types.Var, bool) {
	object, _, _ := types.LookupFieldOrMethod(t, true, pkg, name)
	field, ok := object.(*types.Var)
	return field, ok && field.IsField()
}

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
