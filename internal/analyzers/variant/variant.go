// Package variant enforces TS-V01: every loop that is not structurally
// terminating (RangeStmt) or fixed-count (a Post counter that Cond
// references) must carry a variant — a linear integer ranking expression
// proven to strictly decrease on every back edge and bounded below —
// either synthesized by this analyzer or pinned by the author.
//
// The fact lifecycle is synthesized -> reported -> pinnable. For every
// in-scope loop the analyzer first tries to synthesize a variant from a
// closed set of Cond shapes (len(s) > c, i < n, i > c, low < high) paired
// with an unconditional decrease statement at the top level of the loop
// body (s = s[k:], s = s[:len(s)-k], i++, i += k, low++, high--, ...), with
// nothing anywhere in the body — walked through ifs, switches, and nested
// loops — that could invalidate it: an append into s, any other write to
// the measured variable, &s/&i, a continue that could skip the decrease,
// or a closure that captures it. Synthesis success is a reported fact
// (TS-V01-facts, printed only under --show-facts): the loop is proven with
// no annotation, and the printed line is exact pin syntax
// (directive.FormatVariant), so ENG-151 can freeze it into a
// //tiger:variant pin by pasting it. The same closed set is what can ever
// be reported; it is also all this wave can verify by hand.
//
// A pin replaces synthesis with the author's own ranking, verified by the
// identical discipline: each term of the pinned expression classifies as
// strictly decreasing, strictly increasing, constant, or unknown against
// the same body rules, and the pin verifies only when every positive term
// is non-increasing, every negative term is non-decreasing, at least one
// term moves strictly, and none is unknown. A pin the analyzer cannot
// verify is blocking even when the loop plausibly terminates: a pin
// freezes the analyzer's own computed verification, in both directions —
// the same bidirectional-exactness principle TS-F01 applies to effects —
// so an unverifiable pin is not a weaker claim than an unpinned loop, it is
// a broken contract. A pin on a loop that needs no variant at all (a
// RangeStmt, a fixed-count ForStmt) is blocking for the same reason: it
// states a fact the analyzer never checks, so it means nothing and must be
// removed.
//
// A loop with neither a synthesized nor a verified pinned variant is
// blocking, and the finding always names the same rewrite: an explicit
// iteration cap whose own counter is itself a synthesizable variant, with
// an assert on exhaustion. That rewrite is always available in-dialect, so
// unlike the specification's note that a ranking beyond the predicate
// language "needs a deviation with a reason," this analyzer implements no
// //tiger:variant deviation escape — correctness constraint 7 requires
// every blocking finding to name a code change, never a directive, and the
// counter-cap form is always that change (ADR-0003's escape admission
// test: an escape earns its existence only where no in-dialect rewrite
// exists, and one always exists here).
//
// Scope is AST and go/types only, mirroring boundedloop's structural
// model: a RangeStmt is structurally terminating and out of scope (a stray
// pin on one is itself the finding); a ForStmt with Cond == nil is
// boundedloop's TS-S02/TS-S03 territory; a fixed-count ForStmt (Post
// advances an identifier that Cond references) is bounded by boundedloop,
// not ranked here.
package variant

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/pins"
	"github.com/kapetan-io/tiger/internal/directive"
)

const (
	msgNoVariant = "TS-V01: this loop has no synthesized or pinned variant — no linear ranking " +
		"provably decreases on every back edge; add an explicit iteration cap with an assert on " +
		"exhaustion (for tries := 0; tries < capLimit; tries++ { if done { break } ... }), whose " +
		"counter is itself a synthesizable variant, or pin one with //tiger:variant <expr>"
	msgUnverifiedPinFmt = "TS-V01: the pinned variant //tiger:variant %s cannot be verified to " +
		"strictly decrease on every back edge — %s; rewrite the loop with an explicit iteration " +
		"cap and an assert on exhaustion (for tries := 0; tries < capLimit; tries++), whose " +
		"counter is itself a linear variant, or pin a variant the analyzer can verify"
	msgSynthesizedPrefix = "TS-V01: synthesized variant — //tiger:variant "
	msgPointlessPin      = "TS-V01: this loop needs no variant — it terminates structurally, " +
		"not by a decreasing measure, so the pin states nothing; remove the pin"
)

// Analyzer enforces TS-V01: every unbounded loop has a synthesized or
// verified pinned variant.
var Analyzer = &analysis.Analyzer{
	Name: "variant",
	Doc:  "TS-V01: every loop has a synthesized or verified pinned variant.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	set := pins.Collect(pass.Fset, pass.Files)
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch loop := node.(type) {
			case *ast.RangeStmt:
				checkOutOfScopePin(pass, set, loop)
			case *ast.ForStmt:
				checkFor(pass, set, loop)
			}
			return true
		})
	}
	return nil, nil
}

// checkFor classifies a for statement: out of scope (no Cond, or
// fixed-count — a stray pin there is itself a finding), pinned (verify the
// pin), or unpinned (attempt synthesis).
func checkFor(pass *analysis.Pass, set pins.Set, loop *ast.ForStmt) {
	if loop.Cond == nil {
		checkOutOfScopePin(pass, set, loop)
		return
	}
	if fixedCount(loop) {
		checkOutOfScopePin(pass, set, loop)
		return
	}
	pinned := set.At(loop, "variant")
	if len(pinned) > 0 {
		checkPinned(pass, loop, pinned[0])
		return
	}
	checkUnpinned(pass, loop)
}

// checkOutOfScopePin reports a variant pin bound to a loop that needs no
// variant: a RangeStmt, a ForStmt with no Cond, or a fixed-count ForStmt.
func checkOutOfScopePin(pass *analysis.Pass, set pins.Set, loop ast.Node) {
	if len(set.At(loop, "variant")) == 0 {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-V01",
		Message:  msgPointlessPin,
	})
}

// checkPinned verifies a pinned variant against the loop it binds to,
// reporting a blocking finding only when verification fails.
func checkPinned(pass *analysis.Pass, loop *ast.ForStmt, pin pins.Pin) {
	parsed, err := directive.ParseVariant(pin.Directive.Args)
	if err != nil {
		assert.Unreachable("pins.Collect only yields well-formed variant pins")
		return
	}
	result := verifyPinned(pass, loop, parsed)
	if result.ok {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-V01",
		Message:  fmt.Sprintf(msgUnverifiedPinFmt, directive.FormatVariant(parsed), result.reason),
	})
}

// checkUnpinned attempts synthesis: success is a reported fact in pin
// syntax, failure is a blocking finding naming the counter-cap rewrite.
func checkUnpinned(pass *analysis.Pass, loop *ast.ForStmt) {
	candidate, ok := condMeasure(pass, loop.Cond)
	if ok {
		result := verify(pass, loop.Body, candidate)
		if result.ok {
			pass.Report(analysis.Diagnostic{
				Pos:      loop.Pos(),
				Category: "TS-V01-facts",
				Message:  msgSynthesizedPrefix + directive.FormatVariant(candidate),
			})
			return
		}
	}
	pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-V01",
		Message:  msgNoVariant,
	})
}

// fixedCount reports whether loop's Post advances an identifier that Cond
// references — boundedloop's territory, mirrored locally.
func fixedCount(loop *ast.ForStmt) bool {
	counter := postCounter(loop.Post)
	if counter == "" {
		return false
	}
	return identifierAppears(loop.Cond, counter)
}

// postCounter returns the identifier name a Post clause advances by
// IncDecStmt or by += / -=, or "" when Post proves no counter.
func postCounter(post ast.Stmt) string {
	switch stmt := post.(type) {
	case *ast.IncDecStmt:
		ident, ok := stmt.X.(*ast.Ident)
		if !ok {
			return ""
		}
		return ident.Name
	case *ast.AssignStmt:
		if stmt.Tok != token.ADD_ASSIGN && stmt.Tok != token.SUB_ASSIGN {
			return ""
		}
		if len(stmt.Lhs) != 1 {
			return ""
		}
		ident, ok := stmt.Lhs[0].(*ast.Ident)
		if !ok {
			return ""
		}
		return ident.Name
	}
	return ""
}

// identifierAppears reports whether name is referenced anywhere inside node.
func identifierAppears(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == name {
			found = true
		}
		return true
	})
	return found
}

// bounds is a Cond normalized to low OP high, where OP is < or <=: a > or
// >= comparison is rewritten by swapping its operands.
type bounds struct {
	low  ast.Expr
	high ast.Expr
}

// normalizeCond rewrites a comparison into low/high form, or reports the
// comparison is not one of the four ordering operators.
func normalizeCond(bin *ast.BinaryExpr) (bounds, bool) {
	if bin.Op == token.LSS || bin.Op == token.LEQ {
		return bounds{low: bin.X, high: bin.Y}, true
	}
	if bin.Op == token.GTR || bin.Op == token.GEQ {
		return bounds{low: bin.Y, high: bin.X}, true
	}
	return bounds{}, false
}

// condMeasure classifies cond against the closed recognized-comparison set,
// returning the candidate measure it supplies.
func condMeasure(pass *analysis.Pass, cond ast.Expr) (directive.Variant, bool) {
	binary, ok := cond.(*ast.BinaryExpr)
	if !ok {
		return directive.Variant{}, false
	}
	b, ok := normalizeCond(binary)
	if !ok {
		return directive.Variant{}, false
	}
	if v, ok := lenMeasure(pass, b); ok {
		return v, true
	}
	return linearMeasure(pass, b)
}

// lenMeasure recognizes low OP len(s) with low a non-negative constant and
// s a slice, map, or string identifier: the candidate measure is len(s)
// alone.
func lenMeasure(pass *analysis.Pass, b bounds) (directive.Variant, bool) {
	call, ok := b.high.(*ast.CallExpr)
	if !ok {
		return directive.Variant{}, false
	}
	if !isLenCall(pass, call) {
		return directive.Variant{}, false
	}
	subject, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return directive.Variant{}, false
	}
	if !isLenableType(pass, subject) {
		return directive.Variant{}, false
	}
	if !isNonNegBound(pass, b.low) {
		return directive.Variant{}, false
	}
	term := directive.VariantTerm{Atom: directive.Atom{Kind: directive.AtomLen, Ref: subject.Name}}
	return directive.Variant{Terms: []directive.VariantTerm{term}}, true
}

// scalar is one classified comparison operand: the atom it prints as, and
// whether it is an identifier (as opposed to a literal).
type scalar struct {
	atom  directive.Atom
	ident bool
}

// linearMeasure recognizes an integer comparison between two scalars — each
// an integer-typed identifier or an integer literal — with at least one
// identifier. The candidate measure is high - low, dropping a literal-zero
// low term.
func linearMeasure(pass *analysis.Pass, b bounds) (directive.Variant, bool) {
	low, ok := scalarAtom(pass, b.low)
	if !ok {
		return directive.Variant{}, false
	}
	high, ok := scalarAtom(pass, b.high)
	if !ok {
		return directive.Variant{}, false
	}
	if !low.ident && !high.ident {
		return directive.Variant{}, false
	}
	terms := []directive.VariantTerm{{Atom: high.atom}}
	zero := low.atom.Kind == directive.AtomNumber && low.atom.Int == 0
	if !zero {
		terms = append(terms, directive.VariantTerm{Minus: true, Atom: low.atom})
	}
	return directive.Variant{Terms: terms}, true
}

// scalarAtom classifies expr as an integer-typed identifier or an integer
// literal, the two operand shapes the linear comparison forms admit.
func scalarAtom(pass *analysis.Pass, expr ast.Expr) (scalar, bool) {
	ident, ok := expr.(*ast.Ident)
	if ok {
		if !isIntType(pass, ident) {
			return scalar{}, false
		}
		return scalar{
			atom: directive.Atom{Kind: directive.AtomRef, Ref: ident.Name}, ident: true,
		}, true
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return scalar{}, false
	}
	if lit.Kind != token.INT {
		return scalar{}, false
	}
	number, err := strconv.ParseInt(lit.Value, 0, 64)
	if err != nil {
		return scalar{}, false
	}
	return scalar{atom: directive.Atom{Kind: directive.AtomNumber, Int: number}}, true
}

// isIntType reports whether ident's type is one of the integer kinds.
func isIntType(pass *analysis.Pass, ident *ast.Ident) bool {
	found := pass.TypesInfo.TypeOf(ident)
	if found == nil {
		return false
	}
	basic, ok := found.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	return basic.Info()&types.IsInteger != 0
}

// isLenCall reports whether call is a call to the builtin len with exactly
// one argument.
func isLenCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return false
	}
	if builtin.Name() != "len" {
		return false
	}
	return len(call.Args) == 1
}

// isLenableType reports whether ident's type is a slice, map, or string —
// the len() subjects rule 1 admits.
func isLenableType(pass *analysis.Pass, ident *ast.Ident) bool {
	found := pass.TypesInfo.TypeOf(ident)
	if found == nil {
		return false
	}
	switch under := found.Underlying().(type) {
	case *types.Slice:
		return true
	case *types.Map:
		return true
	case *types.Basic:
		return under.Info()&types.IsString != 0
	}
	return false
}

// isNonNegBound reports whether expr is a constant integer expression
// (literal or named constant) whose value is at least zero.
func isNonNegBound(pass *analysis.Pass, expr ast.Expr) bool {
	found, ok := pass.TypesInfo.Types[expr]
	if !ok {
		return false
	}
	if found.Value == nil {
		return false
	}
	bound, ok := constant.Int64Val(found.Value)
	if !ok {
		return false
	}
	return bound >= 0
}

// trend classifies how a measured variable changes across a loop body.
type trend int

const (
	trendUnknown trend = iota
	trendConstant
	trendIncreasing
	trendDecreasing
)

// verdict is the result of verifying a variant against a loop body.
type verdict struct {
	ok     bool
	reason string
}

// verify checks v against the back-edge discipline: no continue can skip
// the decrease, every positive term is non-increasing, every negative term
// is non-decreasing, at least one term moves strictly, and no term is
// unknown.
func verify(pass *analysis.Pass, body *ast.BlockStmt, v directive.Variant) verdict {
	if hasContinue(body) {
		return verdict{reason: "a continue can skip the loop's decrease"}
	}
	moved := false
	for _, term := range v.Terms {
		t := atomTrend(pass, body, term.Atom)
		if t == trendUnknown {
			return verdict{reason: atomText(term.Atom) + " cannot be shown to move consistently"}
		}
		if term.Minus {
			if t == trendDecreasing {
				return verdict{
					reason: atomText(term.Atom) +
						" decreases, growing the ranking instead of shrinking it",
				}
			}
			if t == trendIncreasing {
				moved = true
			}
			continue
		}
		if t == trendIncreasing {
			return verdict{reason: atomText(term.Atom) + " increases instead of decreasing"}
		}
		if t == trendDecreasing {
			moved = true
		}
	}
	if !moved {
		return verdict{reason: "no term in the expression provably decreases"}
	}
	return verdict{ok: true}
}

// verifyPinned additionally requires the loop's Cond to match one of the
// recognized comparison forms, which supplies the lower bound.
func verifyPinned(pass *analysis.Pass, loop *ast.ForStmt, v directive.Variant) verdict {
	if _, ok := condMeasure(pass, loop.Cond); !ok {
		return verdict{reason: "the loop's condition is not one of the recognized comparison forms"}
	}
	return verify(pass, loop.Body, v)
}

// atomTrend classifies one variant atom's trend within body. A dotted-path
// ref is always unknown: the predicate language admits it syntactically,
// but only locals are classifiable.
func atomTrend(pass *analysis.Pass, body *ast.BlockStmt, atom directive.Atom) trend {
	switch atom.Kind {
	case directive.AtomNumber:
		return trendConstant
	case directive.AtomRef:
		if strings.Contains(atom.Ref, ".") {
			return trendUnknown
		}
		return refTrend(pass, body, atom.Ref)
	case directive.AtomLen:
		if strings.Contains(atom.Ref, ".") {
			return trendUnknown
		}
		return lenTrend(pass, body, atom.Ref)
	case directive.AtomNil:
		return trendUnknown
	default:
		assert.Unreachable("AtomKind: unhandled value")
	}
	return trendUnknown
}

// write is one write statement touching a measured variable, tagged with
// whether it sits directly in the loop body's top-level statement list.
type write struct {
	top        bool
	recognized bool
	delta      int64
}

// topLevelStmts indexes body's direct statements for the top-level check
// the back-edge discipline requires.
func topLevelStmts(body *ast.BlockStmt) map[ast.Stmt]bool {
	top := map[ast.Stmt]bool{}
	for _, stmt := range body.List {
		top[stmt] = true
	}
	return top
}

// refTrend classifies an integer identifier's trend: constant when never
// written, strictly increasing or decreasing when its only write is an
// unconditional top-level ++/--/+=/-= with a positive literal magnitude,
// unknown otherwise — multiple writes, a nested or non-literal write, an
// address taken, or capture by a closure.
func refTrend(pass *analysis.Pass, body *ast.BlockStmt, name string) trend {
	top := topLevelStmts(body)
	captured := false
	addressed := false
	writes := []write{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			if identifierAppears(v.Body, name) {
				captured = true
			}
		case *ast.UnaryExpr:
			if addressOf(v, name) {
				addressed = true
			}
		case *ast.IncDecStmt:
			if w, ok := incDecWrite(v, name, top); ok {
				writes = append(writes, w)
			}
		case *ast.AssignStmt:
			if w, ok := counterAssignWrite(v, name, top); ok {
				writes = append(writes, w)
			}
		}
		return true
	})
	if captured {
		return trendUnknown
	}
	if addressed {
		return trendUnknown
	}
	return writeTrend(writes)
}

// addressOf reports whether expr is &name.
func addressOf(expr *ast.UnaryExpr, name string) bool {
	if expr.Op != token.AND {
		return false
	}
	ident, ok := expr.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == name
}

// incDecWrite records a ++/-- statement touching name.
func incDecWrite(stmt *ast.IncDecStmt, name string, top map[ast.Stmt]bool) (write, bool) {
	ident, ok := stmt.X.(*ast.Ident)
	if !ok {
		return write{}, false
	}
	if ident.Name != name {
		return write{}, false
	}
	delta := int64(1)
	if stmt.Tok == token.DEC {
		delta = -1
	}
	return write{top: top[stmt], recognized: true, delta: delta}, true
}

// counterAssignWrite records an assignment statement touching name,
// recognizing only += / -= by a positive integer literal.
func counterAssignWrite(stmt *ast.AssignStmt, name string, top map[ast.Stmt]bool) (write, bool) {
	if len(stmt.Lhs) != 1 {
		return write{}, false
	}
	ident, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok {
		return write{}, false
	}
	if ident.Name != name {
		return write{}, false
	}
	delta, recognized := assignDelta(stmt)
	return write{top: top[stmt], recognized: recognized, delta: delta}, true
}

// assignDelta reads a += / -= statement's signed magnitude from a positive
// integer literal right-hand side.
func assignDelta(assign *ast.AssignStmt) (int64, bool) {
	if len(assign.Rhs) != 1 {
		return 0, false
	}
	lit, ok := assign.Rhs[0].(*ast.BasicLit)
	if !ok {
		return 0, false
	}
	if lit.Kind != token.INT {
		return 0, false
	}
	magnitude, err := strconv.ParseInt(lit.Value, 0, 64)
	if err != nil {
		return 0, false
	}
	if magnitude < 1 {
		return 0, false
	}
	if assign.Tok == token.ADD_ASSIGN {
		return magnitude, true
	}
	if assign.Tok == token.SUB_ASSIGN {
		return -magnitude, true
	}
	return 0, false
}

// writeTrend reduces a variable's collected writes to its overall trend: no
// writes is constant; exactly one recognized, top-level, literal-magnitude
// write is increasing or decreasing by its sign; anything else is unknown.
func writeTrend(writes []write) trend {
	if len(writes) == 0 {
		return trendConstant
	}
	if len(writes) != 1 {
		return trendUnknown
	}
	w := writes[0]
	if !w.top {
		return trendUnknown
	}
	if !w.recognized {
		return trendUnknown
	}
	if w.delta > 0 {
		return trendIncreasing
	}
	return trendDecreasing
}

// lenTrend classifies a slice/map/string identifier's length trend:
// strictly decreasing when its only body write is an unconditional
// top-level shrink (s = s[k:] or s = s[:len(s)-k], k a positive literal),
// with no append into it, no address taken, and no closure capture;
// constant when untouched; unknown otherwise.
func lenTrend(pass *analysis.Pass, body *ast.BlockStmt, name string) trend {
	top := topLevelStmts(body)
	captured := false
	addressed := false
	appended := false
	writes := []write{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			if identifierAppears(v.Body, name) {
				captured = true
			}
		case *ast.UnaryExpr:
			if addressOf(v, name) {
				addressed = true
			}
		case *ast.CallExpr:
			if isAppendCall(pass, v) && appendTargets(v, name) {
				appended = true
			}
		case *ast.AssignStmt:
			if w, ok := shrinkAssignWrite(v, name, top); ok {
				writes = append(writes, w)
			}
		}
		return true
	})
	if captured {
		return trendUnknown
	}
	if addressed {
		return trendUnknown
	}
	if appended {
		return trendUnknown
	}
	if len(writes) == 0 {
		return trendConstant
	}
	if len(writes) != 1 {
		return trendUnknown
	}
	w := writes[0]
	if !w.top {
		return trendUnknown
	}
	if !w.recognized {
		return trendUnknown
	}
	return trendDecreasing
}

// shrinkAssignWrite records a plain assignment to name, recognizing only
// the two shrink forms s = s[k:] and s = s[:len(s)-k].
func shrinkAssignWrite(stmt *ast.AssignStmt, name string, top map[ast.Stmt]bool) (write, bool) {
	if stmt.Tok != token.ASSIGN {
		return write{}, false
	}
	if len(stmt.Lhs) != 1 {
		return write{}, false
	}
	ident, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok {
		return write{}, false
	}
	if ident.Name != name {
		return write{}, false
	}
	if len(stmt.Rhs) != 1 {
		return write{}, false
	}
	return write{top: top[stmt], recognized: shrinks(stmt.Rhs[0], name)}, true
}

// shrinks reports whether rhs is one of the two recognized shrink forms for
// the identifier name.
func shrinks(rhs ast.Expr, name string) bool {
	slice, ok := rhs.(*ast.SliceExpr)
	if !ok {
		return false
	}
	subject, ok := slice.X.(*ast.Ident)
	if !ok {
		return false
	}
	if subject.Name != name {
		return false
	}
	if slice.Slice3 {
		return false
	}
	if shrinkFromLow(slice) {
		return true
	}
	return shrinkFromHigh(slice, name)
}

// shrinkFromLow matches s[k:] with k a positive integer literal.
func shrinkFromLow(slice *ast.SliceExpr) bool {
	if slice.Low == nil {
		return false
	}
	if slice.High != nil {
		return false
	}
	return positiveLiteral(slice.Low)
}

// shrinkFromHigh matches s[:len(s)-k] with k a positive integer literal.
func shrinkFromHigh(slice *ast.SliceExpr, name string) bool {
	if slice.Low != nil {
		return false
	}
	if slice.High == nil {
		return false
	}
	binary, ok := slice.High.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	if binary.Op != token.SUB {
		return false
	}
	if !isLenOf(binary.X, name) {
		return false
	}
	return positiveLiteral(binary.Y)
}

// isLenOf reports whether expr is a call to len with a single identifier
// argument matching name.
func isLenOf(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	if ident.Name != "len" {
		return false
	}
	if len(call.Args) != 1 {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return false
	}
	return arg.Name == name
}

// positiveLiteral reports whether expr is an integer literal at least 1.
func positiveLiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return false
	}
	if lit.Kind != token.INT {
		return false
	}
	magnitude, err := strconv.ParseInt(lit.Value, 0, 64)
	if err != nil {
		return false
	}
	return magnitude >= 1
}

// isAppendCall reports whether call is a call to the builtin append.
func isAppendCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return false
	}
	return builtin.Name() == "append"
}

// appendTargets reports whether call's first argument is the identifier
// name — the append(s, more) shape that grows s.
func appendTargets(call *ast.CallExpr, name string) bool {
	if len(call.Args) == 0 {
		return false
	}
	ident, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == name
}

// hasContinue reports whether body contains a continue that could target
// this loop — any continue not itself inside a nested for/range, since an
// unlabeled continue there targets the inner loop instead.
func hasContinue(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.ForStmt:
			return false
		case *ast.RangeStmt:
			return false
		case *ast.BranchStmt:
			if v.Tok == token.CONTINUE {
				found = true
			}
		}
		return true
	})
	return found
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
