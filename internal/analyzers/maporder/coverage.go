package maporder

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type totalFact struct{}

func (*totalFact) AFact()         {}
func (*totalFact) String() string { return "total" }

type totalCandidate struct {
	method *types.Func
	result ast.Expr
	ops    comparands
	typ    types.Type
}

func exportTotalFacts(pass *analysis.Pass) {
	candidates := totalCandidates(pass)
	for range len(candidates) {
		added := false
		for i, cand := range candidates {
			if cand.method == nil || !totalOver(pass, cand.typ, cand.result, cand.ops) {
				continue
			}
			pass.ExportObjectFact(cand.method, &totalFact{})
			candidates[i].method = nil
			added = true
		}
		if !added {
			return
		}
	}
}

func totalCandidates(pass *analysis.Pass) []totalCandidate {
	var found []totalCandidate
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || len(fd.Body.List) != 1 {
				continue
			}
			recv, param := soleName(fd.Recv), soleName(fd.Type.Params)
			if recv == nil || param == nil || !returnsInt(pass, fd) {
				continue
			}
			ret, ok := fd.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			a, b := pass.TypesInfo.Defs[recv], pass.TypesInfo.Defs[param]
			method, isFunc := pass.TypesInfo.Defs[fd.Name].(*types.Func)
			if a == nil || b == nil || !isFunc || !types.Identical(a.Type(), b.Type()) {
				continue
			}
			ops := comparands{a, b}
			found = append(found, totalCandidate{method, ret.Results[0], ops, a.Type()})
		}
	}
	return found
}

func soleName(fields *ast.FieldList) *ast.Ident {
	if fields == nil || len(fields.List) != 1 || len(fields.List[0].Names) != 1 {
		return nil
	}
	return fields.List[0].Names[0]
}

func returnsInt(pass *analysis.Pass, fd *ast.FuncDecl) bool {
	fn, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func)
	if !ok {
		return false
	}
	results := fn.Signature().Results()
	return results.Len() == 1 && types.Identical(results.At(0).Type(), types.Typ[types.Int])
}

func totalComparator(pass *analysis.Pass, fn ast.Expr, elem types.Type) bool {
	if elem == nil {
		return false
	}
	switch f := ast.Unparen(fn).(type) {
	case *ast.FuncLit:
		var names []*ast.Ident
		for _, field := range f.Type.Params.List {
			names = append(names, field.Names...)
		}
		if len(names) != 2 || len(f.Body.List) != 1 {
			return false
		}
		ret, ok := f.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return false
		}
		a, b := pass.TypesInfo.Defs[names[0]], pass.TypesInfo.Defs[names[1]]
		if a == nil || b == nil {
			return false
		}
		if !types.Identical(a.Type(), elem) || !types.Identical(b.Type(), elem) {
			return false
		}
		return totalOver(pass, elem, ret.Results[0], comparands{a, b})
	case *ast.SelectorExpr:
		sel := pass.TypesInfo.Selections[f]
		if sel == nil || sel.Kind() != types.MethodExpr || !types.Identical(sel.Recv(), elem) {
			return false
		}
		return pass.ImportObjectFact(sel.Obj(), new(totalFact))
	}
	return false
}

type comparands struct{ first, second types.Object }

type sides struct{ left, right ast.Expr }

type leafPath struct {
	path string
	typ  types.Type
}

const maxPathDepth = 64

func totalOver(pass *analysis.Pass, elem types.Type, expr ast.Expr, ops comparands) bool {
	comps := []ast.Expr{expr}
	if call, ok := ast.Unparen(expr).(*ast.CallExpr); ok && isFunc(pass, call.Fun, "cmp.Or") {
		if call.Ellipsis.IsValid() {
			return false
		}
		comps = call.Args
	}
	covered := map[string]types.Type{}
	for _, comp := range comps {
		leaf, ok := component(pass, comp, ops)
		if !ok {
			return false
		}
		if leaf.path != "" {
			covered[leaf.path] = leaf.typ
		}
	}
	return covers(elem, covered)
}

func component(pass *analysis.Pass, comp ast.Expr, ops comparands) (leafPath, bool) {
	comp = ast.Unparen(comp)
	if neg, ok := comp.(*ast.UnaryExpr); ok && neg.Op == token.SUB {
		comp = ast.Unparen(neg.X)
	}
	call, ok := comp.(*ast.CallExpr)
	if !ok || call.Ellipsis.IsValid() {
		return leafPath{}, false
	}
	if leaf, ok := methodComparison(pass, call, ops); ok {
		return leaf, true
	}
	if len(call.Args) != 2 {
		return leafPath{}, false
	}
	operands := sides{call.Args[0], call.Args[1]}
	switch {
	case isFunc(pass, call.Fun, "cmp.Compare"), isFunc(pass, call.Fun, "strings.Compare"):
		leftType := pass.TypesInfo.TypeOf(operands.left)
		if path, ok := pairedPath(pass, operands, ops); ok && orderedLeaf(leftType) {
			return leafPath{path, leftType}, true
		}
	case isFunc(pass, call.Fun, "bytes.Compare"), isFunc(pass, call.Fun, "slices.Compare"):
		if leaf, ok := wholeArray(pass, operands, ops); ok {
			return leaf, true
		}
	default:
		return leafPath{}, false
	}
	return leafPath{}, mirrored(pass, operands, ops)
}

func methodComparison(pass *analysis.Pass, call *ast.CallExpr, ops comparands) (leafPath, bool) {
	fun, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 1 {
		return leafPath{}, false
	}
	sel := pass.TypesInfo.Selections[fun]
	if sel == nil || sel.Kind() != types.MethodVal || len(sel.Index()) != 1 || sel.Indirect() {
		return leafPath{}, false
	}
	if !pass.ImportObjectFact(sel.Obj(), new(totalFact)) {
		return leafPath{}, false
	}
	method, ok := sel.Obj().(*types.Func)
	if !ok {
		return leafPath{}, false
	}
	recv := method.Signature().Recv().Type()
	if !types.Identical(pass.TypesInfo.TypeOf(fun.X), recv) {
		return leafPath{}, false
	}
	path, ok := pairedPath(pass, sides{fun.X, call.Args[0]}, ops)
	return leafPath{path, recv}, ok
}

func wholeArray(pass *analysis.Pass, operands sides, ops comparands) (leafPath, bool) {
	xs, ok1 := ast.Unparen(operands.left).(*ast.SliceExpr)
	ys, ok2 := ast.Unparen(operands.right).(*ast.SliceExpr)
	if !ok1 || !ok2 || !fullSlice(xs) || !fullSlice(ys) {
		return leafPath{}, false
	}
	array, ok := arrayOf(pass, xs.X)
	if !ok || !orderedLeaf(array.Elem()) {
		return leafPath{}, false
	}
	path, ok := pairedPath(pass, sides{xs.X, ys.X}, ops)
	return leafPath{path, pass.TypesInfo.TypeOf(xs.X)}, ok
}

func pairedPath(pass *analysis.Pass, operands sides, ops comparands) (string, bool) {
	for _, roots := range [][2]types.Object{{ops.first, ops.second}, {ops.second, ops.first}} {
		left, okLeft := projection(pass, operands.left, roots[0])
		right, okRight := projection(pass, operands.right, roots[1])
		if okLeft && okRight && left == right {
			return left, true
		}
	}
	return "", false
}

func projection(pass *analysis.Pass, expr ast.Expr, root types.Object) (string, bool) {
	var steps []string
	for range maxPathDepth {
		switch e := ast.Unparen(expr).(type) {
		case *ast.Ident:
			if pass.TypesInfo.Uses[e] != root {
				return "", false
			}
			slices.Reverse(steps)
			return "$" + strings.Join(steps, ""), true
		case *ast.SelectorExpr:
			sel := pass.TypesInfo.Selections[e]
			if sel == nil || sel.Kind() != types.FieldVal || sel.Indirect() {
				return "", false
			}
			step, ok := fieldStep(sel, pass.TypesInfo.TypeOf(e.X))
			if !ok {
				return "", false
			}
			steps = append(steps, step)
			expr = e.X
		case *ast.IndexExpr:
			tv := pass.TypesInfo.Types[e.Index]
			if _, ok := arrayOf(pass, e.X); !ok || tv.Value == nil {
				return "", false
			}
			steps = append(steps, "["+tv.Value.ExactString()+"]")
			expr = e.X
		default:
			return "", false
		}
	}
	return "", false
}

func fieldStep(sel *types.Selection, holder types.Type) (string, bool) {
	var step strings.Builder
	for _, index := range sel.Index() {
		st, ok := holder.Underlying().(*types.Struct)
		if !ok {
			return "", false
		}
		field := st.Field(index)
		step.WriteString("." + field.Name())
		holder = field.Type()
	}
	return step.String(), true
}

func mirrored(pass *analysis.Pass, operands sides, ops comparands) bool {
	x, y, a, b := operands.left, operands.right, ops.first, ops.second
	readsA := readsAny(pass, x, map[types.Object]bool{a: true})
	readsB := readsAny(pass, x, map[types.Object]bool{b: true})
	if readsA == readsB || !pureExpr(pass, x) {
		return false
	}
	swap := func(obj types.Object) types.Object {
		switch obj {
		case a:
			return b
		case b:
			return a
		}
		return obj
	}
	var walk func(x, y ast.Node) bool
	walk = func(x, y ast.Node) bool {
		switch xn := x.(type) {
		case *ast.Ident:
			yn, ok := y.(*ast.Ident)
			return ok && swap(pass.TypesInfo.ObjectOf(xn)) == pass.TypesInfo.ObjectOf(yn)
		case *ast.BasicLit:
			yn, ok := y.(*ast.BasicLit)
			return ok && xn.Kind == yn.Kind && xn.Value == yn.Value
		case *ast.ParenExpr:
			yn, ok := y.(*ast.ParenExpr)
			return ok && walk(xn.X, yn.X)
		case *ast.SelectorExpr:
			yn, ok := y.(*ast.SelectorExpr)
			return ok && xn.Sel.Name == yn.Sel.Name && walk(xn.X, yn.X)
		case *ast.IndexExpr:
			yn, ok := y.(*ast.IndexExpr)
			return ok && walk(xn.X, yn.X) && walk(xn.Index, yn.Index)
		case *ast.StarExpr:
			yn, ok := y.(*ast.StarExpr)
			return ok && walk(xn.X, yn.X)
		case *ast.UnaryExpr:
			yn, ok := y.(*ast.UnaryExpr)
			return ok && xn.Op == yn.Op && walk(xn.X, yn.X)
		case *ast.BinaryExpr:
			yn, ok := y.(*ast.BinaryExpr)
			return ok && xn.Op == yn.Op && walk(xn.X, yn.X) && walk(xn.Y, yn.Y)
		case *ast.CallExpr:
			yn, ok := y.(*ast.CallExpr)
			if !ok || len(xn.Args) != len(yn.Args) || !walk(xn.Fun, yn.Fun) {
				return false
			}
			for i := range xn.Args {
				if !walk(xn.Args[i], yn.Args[i]) {
					return false
				}
			}
			return true
		}
		return false
	}
	return walk(x, y)
}

const maxCoverDepth = 8

const maxCoverArray = 64

const maxCoverParts = 4096

func covers(t types.Type, covered map[string]types.Type) bool {
	type part struct {
		typ   types.Type
		path  string
		depth int
	}
	work := []part{{t, "$", 0}}
	for range maxCoverParts {
		if len(work) == 0 {
			return true
		}
		item := work[len(work)-1]
		work = work[:len(work)-1]
		if leaf, ok := covered[item.path]; ok && types.Identical(leaf, item.typ) {
			continue
		}
		if item.depth > maxCoverDepth {
			return false
		}
		switch u := item.typ.Underlying().(type) {
		case *types.Struct:
			for f := range u.NumFields() {
				field := u.Field(f)
				if field.Name() != "_" {
					name := item.path + "." + field.Name()
					work = append(work, part{field.Type(), name, item.depth + 1})
				}
			}
		case *types.Array:
			if u.Len() > maxCoverArray {
				return false
			}
			for e := range u.Len() {
				name := fmt.Sprintf("%s[%d]", item.path, e)
				work = append(work, part{u.Elem(), name, item.depth + 1})
			}
		default:
			return false
		}
	}
	return false
}

func fullSlice(s *ast.SliceExpr) bool { return s.Low == nil && s.High == nil && s.Max == nil }

func arrayOf(pass *analysis.Pass, expr ast.Expr) (*types.Array, bool) {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return nil, false
	}
	array, ok := t.Underlying().(*types.Array)
	return array, ok
}
