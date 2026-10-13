package limitflow

import (
	"go/ast"
	"go/types"
	"slices"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/counter"
)

// sinkKind says where a value becomes a loop limit.
type sinkKind int

const (
	// loopLimit is the limit in a loop's own header.
	loopLimit sinkKind = iota
	// callArg is an argument passed to a parameter that bounds a loop.
	callArg
	// fieldWrite is a value written into a field that bounds a loop.
	fieldWrite
)

// directness says whether a traced value is the loop's own limit or has
// already passed through a variable, a call, or a write. Only a value read
// straight into the loop's header teaches the trace which fields bound a
// loop.
type directness bool

const (
	direct   directness = true
	indirect directness = false
)

// sink is one value that may become a loop limit.
type sink struct {
	kind   sinkKind
	expr   ast.Expr
	mode   directness
	callee *types.Func
	index  int
	field  *types.Var
}

// active reports whether the sink feeds a loop given the facts so far: a
// loop's own limit always does, an argument does when its parameter bounds a
// loop, and a write does when its field bounds one.
func (s *state) active(found sink) bool {
	if found.kind == callArg {
		return slices.Contains(s.limitParamsOf(found.callee), found.index)
	}
	if found.kind == fieldWrite {
		return s.limitFields[s.fieldKey(found.field)]
	}
	return true
}

// sinksIn lists every place in fn where a value can become a loop limit.
func (s *state) sinksIn(fn *function) []sink {
	var found []sink
	ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			found = append(found, s.limitsOf(fn, node)...)
		case *ast.RangeStmt:
			if isInteger(s.pass.TypesInfo.TypeOf(node.X)) {
				found = append(found, sink{kind: loopLimit, expr: node.X, mode: direct})
			}
		case *ast.CallExpr:
			found = append(found, s.argumentsOf(node)...)
		case *ast.CompositeLit:
			found = append(found, s.literalFields(node)...)
		case *ast.AssignStmt:
			found = append(found, s.assignedFields(node)...)
		}
		return true
	})
	return found
}

// limitsOf returns the limit of each sound counter comparison in a loop.
func (s *state) limitsOf(fn *function, loop *ast.ForStmt) []sink {
	var found []sink
	for _, term := range counter.Terms(fn.scope, s.sizes(), loop) {
		found = append(found, sink{kind: loopLimit, expr: term.Limit, mode: direct})
	}
	return found
}

// argumentsOf returns each argument of a call to a declared function or
// method, which bounds a loop when the callee's parameter does.
func (s *state) argumentsOf(call *ast.CallExpr) []sink {
	callee := s.callee(call)
	if callee == nil || call.Ellipsis.IsValid() {
		return nil
	}
	var found []sink
	for i, arg := range call.Args {
		found = append(found, sink{
			kind: callArg, expr: arg, mode: indirect, callee: callee, index: i,
		})
	}
	return found
}

// callee resolves a call to the declared function or method it names. A call
// through a function value or an interface method resolves to nothing the
// trace can follow past the interface, which is a recorded miss.
func (s *state) callee(call *ast.CallExpr) *types.Func {
	var name *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		name = fun
	case *ast.SelectorExpr:
		name = fun.Sel
	default:
		return nil
	}
	fn, ok := s.pass.TypesInfo.Uses[name].(*types.Func)
	if !ok {
		return nil
	}
	return fn.Origin()
}

// literalFields returns each keyed value of a struct literal.
func (s *state) literalFields(literal *ast.CompositeLit) []sink {
	var found []sink
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if field, isVar := s.pass.TypesInfo.Uses[key].(*types.Var); isVar && field.IsField() {
			found = append(found, sink{kind: fieldWrite, expr: pair.Value, mode: indirect, field: field})
		}
	}
	return found
}

// assignedFields returns each value assigned straight into a struct field.
func (s *state) assignedFields(assign *ast.AssignStmt) []sink {
	if len(assign.Lhs) != len(assign.Rhs) {
		return nil
	}
	var found []sink
	for i, lhs := range assign.Lhs {
		selector, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		selection := s.pass.TypesInfo.Selections[selector]
		if selection == nil || selection.Kind() != types.FieldVal {
			continue
		}
		if field, isVar := selection.Obj().(*types.Var); isVar {
			found = append(found, sink{kind: fieldWrite, expr: assign.Rhs[i], mode: indirect, field: field})
		}
	}
	return found
}

// fieldKey names a struct field the same way in every package: the path of
// the package that declares the struct, the struct's name, the field's name.
func (s *state) fieldKey(field *types.Var) string {
	field = field.Origin()
	if field.Pkg() == nil {
		return field.Name()
	}
	scope := field.Pkg().Scope()
	for _, name := range scope.Names() {
		declared, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		fields, ok := declared.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < fields.NumFields(); i++ {
			if fields.Field(i) == field {
				return field.Pkg().Path() + "." + name + "." + field.Name()
			}
		}
	}
	return field.Pkg().Path() + ".?." + field.Name()
}
