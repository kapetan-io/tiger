// Package limitfacts prototypes approach (A): every parameter or struct field
// that bounds a counter loop carries a fact across packages, and every value
// that flows into one must be a constant, another bounded limit, or clamped
// against a declared maximum before it arrives.
package limitfacts

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
	"os"
	"sort"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

// LimitParams marks the parameters of a function that bound a loop.
type LimitParams struct{ Index []int }

func (*LimitParams) AFact()           {}
func (f *LimitParams) String() string { return fmt.Sprint(f.Index) }

// LimitFields lists struct fields (pkgpath.Type.Field) that bound a loop in
// this package. A package fact, because the field may belong to another package.
type LimitFields struct{ Keys []string }

func (*LimitFields) AFact()           {}
func (f *LimitFields) String() string { return fmt.Sprint(f.Keys) }

var Analyzer = &analysis.Analyzer{
	Name:      "limitfacts",
	Doc:       "every value reaching a loop limit is a constant, a bounded limit, or clamped",
	Run:       run,
	FactTypes: []analysis.Fact{new(LimitParams), new(LimitFields)},
}

// Stats is filled per run for measurement.
var Stats = stats{}

type stats struct{}

func (stats) inc(k string, pos token.Position) {
	if os.Getenv("LF_STATS") != "" {
		fmt.Fprintf(os.Stderr, "STAT %s %s\n", k, pos)
	}
}

type state struct {
	pass   *analysis.Pass
	params map[*types.Func]map[int]bool
	fields map[string]bool
	local  map[string]bool // fields discovered in this package
	decls  map[*types.Func]*ast.FuncDecl
}

func run(pass *analysis.Pass) (any, error) {
	s := &state{pass: pass, params: map[*types.Func]map[int]bool{}, fields: map[string]bool{}, local: map[string]bool{}, decls: map[*types.Func]*ast.FuncDecl{}}
	for _, pf := range pass.AllPackageFacts() {
		if lf, ok := pf.Fact.(*LimitFields); ok {
			for _, k := range lf.Keys {
				s.fields[k] = true
			}
		}
	}
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				if fn, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func); ok {
					s.decls[fn] = fd
				}
			}
		}
	}
	for fn, fd := range s.decls {
		s.findLoopLimits(fn, fd)
	}
	// Forwarding to a fixpoint, then report.
	for changed := true; changed; {
		changed = s.checkSinks(false)
	}
	s.checkSinks(true)
	for fn, idx := range s.params {
		var list []int
		for i := range idx {
			list = append(list, i)
		}
		sort.Ints(list)
		pass.ExportObjectFact(fn, &LimitParams{Index: list})
	}
	var keys []string
	for k := range s.local {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		pass.ExportPackageFact(&LimitFields{Keys: keys})
	}
	return nil, nil
}

func fieldKey(v *types.Var) string {
	v = v.Origin()
	if v.Pkg() == nil {
		return v.Name()
	}
	scope := v.Pkg().Scope()
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i) == v {
				return v.Pkg().Path() + "." + name + "." + v.Name()
			}
		}
	}
	return v.Pkg().Path() + ".?." + v.Name()
}

// limitExpr records a loop limit expression X found in fn.
func (s *state) recordLimit(fn *types.Func, x ast.Expr) {
	x = ast.Unparen(x)
	if conv, ok := x.(*ast.CallExpr); ok && len(conv.Args) == 1 {
		if tv := s.pass.TypesInfo.Types[conv.Fun]; tv.IsType() {
			x = ast.Unparen(conv.Args[0])
		}
	}
	switch e := x.(type) {
	case *ast.Ident:
		if i := paramIndex(s.pass, fn, e); i >= 0 {
			s.addParam(fn, i)
			Stats.inc("limit:param", s.pass.Fset.Position(x.Pos()))
			return
		}
	case *ast.SelectorExpr:
		if sel := s.pass.TypesInfo.Selections[e]; sel != nil && sel.Kind() == types.FieldVal {
			k := fieldKey(sel.Obj().(*types.Var))
			if !s.fields[k] {
				s.fields[k] = true
				s.local[k] = true
			}
			Stats.inc("limit:field", s.pass.Fset.Position(x.Pos()))
			return
		}
	}
	if tv := s.pass.TypesInfo.Types[x]; tv.Value != nil {
		Stats.inc("limit:const", s.pass.Fset.Position(x.Pos()))
		return
	}
	if c, ok := x.(*ast.CallExpr); ok {
		if id, ok := c.Fun.(*ast.Ident); ok && (id.Name == "len" || id.Name == "cap") {
			Stats.inc("limit:len", s.pass.Fset.Position(x.Pos()))
			return
		}
	}
	Stats.inc("limit:other", s.pass.Fset.Position(x.Pos()))
}

func (s *state) addParam(fn *types.Func, i int) bool {
	if s.params[fn] == nil {
		s.params[fn] = map[int]bool{}
	}
	if s.params[fn][i] {
		return false
	}
	s.params[fn][i] = true
	return true
}

// findLoopLimits finds `for c := ..; c < X; c++` and, inside any loop whose
// body increments c, `if c >= X { return | break }`.
func (s *state) findLoopLimits(fn *types.Func, fd *ast.FuncDecl) {
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch loop := n.(type) {
		case *ast.ForStmt:
			body = loop.Body
			if inc, ok := loop.Post.(*ast.IncDecStmt); ok && inc.Tok == token.INC && loop.Cond != nil {
				if ctr, ok := inc.X.(*ast.Ident); ok {
					for _, part := range splitAnd(loop.Cond) {
						if b, ok := part.(*ast.BinaryExpr); ok && (b.Op == token.LSS || b.Op == token.LEQ) && sameVar(s.pass, b.X, ctr) {
							s.recordLimit(fn, b.Y)
						}
					}
				}
			}
		case *ast.RangeStmt:
			body = loop.Body
			if b, ok := s.pass.TypesInfo.TypeOf(loop.X).Underlying().(*types.Basic); ok && b.Info()&types.IsInteger != 0 {
				s.recordLimit(fn, loop.X)
			}
		default:
			return true
		}
		incremented := map[types.Object]bool{}
		ast.Inspect(body, func(m ast.Node) bool {
			if inc, ok := m.(*ast.IncDecStmt); ok && inc.Tok == token.INC {
				if id, ok := inc.X.(*ast.Ident); ok {
					incremented[s.pass.TypesInfo.ObjectOf(id)] = true
				}
			}
			return true
		})
		for _, st := range body.List {
			ifs, ok := st.(*ast.IfStmt)
			if !ok || len(ifs.Body.List) == 0 {
				continue
			}
			b, ok := ast.Unparen(ifs.Cond).(*ast.BinaryExpr)
			if !ok || (b.Op != token.GEQ && b.Op != token.EQL && b.Op != token.GTR) {
				continue
			}
			id, ok := b.X.(*ast.Ident)
			if !ok || !incremented[s.pass.TypesInfo.ObjectOf(id)] {
				continue
			}
			switch last := ifs.Body.List[len(ifs.Body.List)-1].(type) {
			case *ast.ReturnStmt:
				s.recordLimit(fn, b.Y)
			case *ast.BranchStmt:
				if last.Tok == token.BREAK {
					s.recordLimit(fn, b.Y)
				}
			}
		}
		return true
	})
}

// checkSinks visits every value flowing into a limit. With report false it
// only propagates forwarding facts and returns whether any were added.
func (s *state) checkSinks(report bool) bool {
	changed := false
	for fn, fd := range s.decls {
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch e := n.(type) {
			case *ast.CallExpr:
				target := callee(s.pass, e)
				if target == nil {
					return true
				}
				for _, i := range s.limitParams(target) {
					if i < len(e.Args) {
						changed = s.sink(fn, fd, e.Args[i], target.Name(), report) || changed
					}
				}
			case *ast.CompositeLit:
				for _, el := range e.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok {
						continue
					}
					if v, ok := s.pass.TypesInfo.Uses[key].(*types.Var); ok && v.IsField() && s.fields[fieldKey(v)] {
						changed = s.sink(fn, fd, kv.Value, "field "+v.Name(), report) || changed
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range e.Lhs {
					se, ok := lhs.(*ast.SelectorExpr)
					if !ok || i >= len(e.Rhs) || len(e.Lhs) != len(e.Rhs) {
						continue
					}
					if sel := s.pass.TypesInfo.Selections[se]; sel != nil && sel.Kind() == types.FieldVal && s.fields[fieldKey(sel.Obj().(*types.Var))] {
						changed = s.sink(fn, fd, e.Rhs[i], "field "+se.Sel.Name, report) || changed
					}
				}
			}
			return true
		})
	}
	return changed
}

func (s *state) limitParams(fn *types.Func) []int {
	if idx, ok := s.params[fn]; ok {
		var out []int
		for i := range idx {
			out = append(out, i)
		}
		return out
	}
	var fact LimitParams
	if s.pass.ImportObjectFact(fn, &fact) {
		return fact.Index
	}
	return nil
}

func (s *state) sink(fn *types.Func, fd *ast.FuncDecl, arg ast.Expr, what string, report bool) bool {
	verdict, fwd := s.bounded(fn, fd, arg)
	changed := false
	if fwd >= 0 {
		changed = s.addParam(fn, fwd)
	}
	if report {
		Stats.inc("sink:"+verdict, s.pass.Fset.Position(arg.Pos()))
		switch verdict {
		case "huge":
			s.pass.Reportf(arg.Pos(), "TS-S02: %s bounds a loop, and this value is too large for the loop ever to reach; pass a declared limit", what)
		case "unbounded":
			s.pass.Reportf(arg.Pos(), "TS-S02: %s bounds a loop, and this value has no upper bound where it enters; clamp it where it is read, such as min(x, xMax)", what)
		}
	}
	return changed
}

// bounded classifies a value: "const", "huge", "len", "limit" (another bounded
// limit), "forward" (a parameter of fn, which now carries the fact),
// "clamped", or "unbounded".
func (s *state) bounded(fn *types.Func, fd *ast.FuncDecl, arg ast.Expr) (string, int) {
	arg = ast.Unparen(arg)
	tv := s.pass.TypesInfo.Types[arg]
	if tv.Value != nil {
		if huge(s.pass, tv) {
			return "huge", -1
		}
		return "const", -1
	}
	switch e := arg.(type) {
	case *ast.CallExpr:
		if s.pass.TypesInfo.Types[e.Fun].IsType() && len(e.Args) == 1 {
			return s.bounded(fn, fd, e.Args[0])
		}
		if id, ok := e.Fun.(*ast.Ident); ok {
			switch id.Name {
			case "len", "cap":
				return "len", -1
			case "min":
				for _, a := range e.Args {
					if v, _ := s.bounded(fn, fd, a); v == "const" || v == "len" || v == "limit" {
						return "clamped", -1
					}
				}
			}
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM, token.SHR:
			x, fx := s.bounded(fn, fd, e.X)
			y, fy := s.bounded(fn, fd, e.Y)
			if x != "unbounded" && x != "huge" && y != "unbounded" && y != "huge" {
				return "clamped", max(fx, fy)
			}
		}
	case *ast.Ident:
		if i := paramIndex(s.pass, fn, e); i >= 0 && !s.clampedBefore(fd, arg) {
			return "forward", i
		}
	case *ast.SelectorExpr:
		if sel := s.pass.TypesInfo.Selections[e]; sel != nil && sel.Kind() == types.FieldVal && s.fields[fieldKey(sel.Obj().(*types.Var))] {
			return "limit", -1
		}
	}
	if s.clampedBefore(fd, arg) {
		return "clamped", -1
	}
	return "unbounded", -1
}

// clampedBefore reports whether a statement before the sink, in an enclosing
// block, compares the same expression against a constant and returns or
// reassigns it: `if x > xMax { return err }` / `{ x = xMax }`, or `x = min(x, xMax)`.
func (s *state) clampedBefore(fd *ast.FuncDecl, arg ast.Expr) bool {
	want := types.ExprString(arg)
	path, _ := astutil.PathEnclosingInterval(fileOf(s.pass, fd), arg.Pos(), arg.End())
	for i := 0; i+1 < len(path); i++ {
		block, ok := path[i+1].(*ast.BlockStmt)
		if !ok {
			continue
		}
		for _, st := range block.List {
			if st.Pos() >= path[i].Pos() {
				break
			}
			if s.isClamp(st, want) {
				return true
			}
		}
	}
	return false
}

func (s *state) isClamp(st ast.Stmt, want string) bool {
	switch st := st.(type) {
	case *ast.AssignStmt:
		if len(st.Lhs) == 1 && types.ExprString(st.Lhs[0]) == want {
			if c, ok := st.Rhs[0].(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "min" {
					return true
				}
			}
		}
	case *ast.IfStmt:
		for _, part := range splitOr(st.Cond) {
			b, ok := part.(*ast.BinaryExpr)
			if !ok || (b.Op != token.GTR && b.Op != token.GEQ) {
				continue
			}
			if stripConv(s.pass, b.X) != want || s.pass.TypesInfo.Types[b.Y].Value == nil {
				continue
			}
			if len(st.Body.List) == 0 {
				continue
			}
			switch last := st.Body.List[len(st.Body.List)-1].(type) {
			case *ast.ReturnStmt:
				return true
			case *ast.AssignStmt:
				return types.ExprString(last.Lhs[0]) == want
			case *ast.ExprStmt:
				if c, ok := last.X.(*ast.CallExpr); ok {
					if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "panic" {
						return true
					}
				}
			}
		}
	}
	return false
}

func stripConv(pass *analysis.Pass, e ast.Expr) string {
	if c, ok := ast.Unparen(e).(*ast.CallExpr); ok && len(c.Args) == 1 && pass.TypesInfo.Types[c.Fun].IsType() {
		return types.ExprString(c.Args[0])
	}
	return types.ExprString(e)
}

func fileOf(pass *analysis.Pass, fd *ast.FuncDecl) *ast.File {
	for _, f := range pass.Files {
		if f.Pos() <= fd.Pos() && fd.End() <= f.End() {
			return f
		}
	}
	return nil
}

func huge(pass *analysis.Pass, tv types.TypeAndValue) bool {
	if tv.Value.Kind() != constant.Int {
		return false
	}
	basic, ok := tv.Type.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsInteger == 0 {
		basic = types.Typ[types.Int]
	}
	bits := pass.TypesSizes.Sizeof(basic) * 8
	if basic.Info()&types.IsUnsigned == 0 {
		bits--
	}
	half := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	v, ok := new(big.Int).SetString(tv.Value.ExactString(), 10)
	return ok && v.Cmp(half) >= 0
}

func callee(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	}
	if id == nil {
		return nil
	}
	fn, _ := pass.TypesInfo.Uses[id].(*types.Func)
	if fn != nil {
		fn = fn.Origin()
	}
	return fn
}

func paramIndex(pass *analysis.Pass, fn *types.Func, e ast.Expr) int {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return -1
	}
	params := fn.Type().(*types.Signature).Params()
	for i := 0; i < params.Len(); i++ {
		if pass.TypesInfo.Uses[id] == params.At(i) {
			return i
		}
	}
	return -1
}

func sameVar(pass *analysis.Pass, e ast.Expr, id *ast.Ident) bool {
	x, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.ObjectOf(x) == pass.TypesInfo.ObjectOf(id)
}

func splitAnd(e ast.Expr) []ast.Expr {
	e = ast.Unparen(e)
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.LAND {
		return append(splitAnd(b.X), splitAnd(b.Y)...)
	}
	return []ast.Expr{e}
}

func splitOr(e ast.Expr) []ast.Expr {
	e = ast.Unparen(e)
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.LOR {
		return append(splitOr(b.X), splitOr(b.Y)...)
	}
	return []ast.Expr{e}
}
