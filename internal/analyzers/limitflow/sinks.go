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

// reach says whether a traced value is read straight from a loop's header
// (atHeader) or arrived through a variable, a call, or a write (downstream).
// Only a value read atHeader teaches the trace which fields bound a loop.
type reach bool

const (
	atHeader   reach = true
	downstream reach = false
)

// sink is one value that may become a loop limit.
type sink struct {
	kind   sinkKind
	expr   ast.Expr
	mode   reach
	callee *types.Func
	index  int
	field  *types.Var
}

// active reports whether the sink feeds a loop given the facts so far: a
// loop's own limit always does, an argument does when its parameter bounds a
// loop, and a write does when its field bounds one.
func (s *state) active(candidate sink) bool {
	if candidate.kind == callArg {
		return slices.Contains(s.limitParamsOf(candidate.callee), candidate.index)
	}
	if candidate.kind == fieldWrite {
		key := s.fieldKey(candidate.field)
		return key != "" && s.limitFields[key]
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
				found = append(found, sink{kind: loopLimit, expr: node.X, mode: atHeader})
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
	for _, term := range counter.Terms(fn.scope, fn.scope.Sizes(), loop) {
		found = append(found, sink{kind: loopLimit, expr: term.Limit, mode: atHeader})
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
	offset := s.receiverArguments(call)
	var found []sink
	for i, arg := range call.Args {
		if i < offset {
			continue
		}
		found = append(found, sink{
			kind: callArg, expr: arg, mode: downstream, callee: callee, index: i - offset,
		})
	}
	return found
}

// receiverArguments returns how many leading arguments of a call are the
// receiver rather than a parameter: one for a method expression such as
// Spinner.Spin(s, limit), none for any other call.
func (s *state) receiverArguments(call *ast.CallExpr) int {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return 0
	}
	selection := s.pass.TypesInfo.Selections[selector]
	if selection != nil && selection.Kind() == types.MethodExpr {
		return 1
	}
	return 0
}

// callee resolves a call to the declared function or method it names. A call
// through a function value or an interface method returns nil, a recorded
// miss.
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
			found = append(found, sink{kind: fieldWrite, expr: pair.Value, mode: downstream, field: field})
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
			found = append(found, sink{kind: fieldWrite, expr: assign.Rhs[i], mode: downstream, field: field})
		}
	}
	return found
}

// fieldKey names a struct field the same way in every package: the path of
// the package that declares the struct, the struct's name, the field's name.
// A field of an anonymous or function-local struct has no such name, so it
// gets none and no fact is kept for it.
func (s *state) fieldKey(field *types.Var) string {
	field = field.Origin()
	if field.Pkg() == nil {
		return ""
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
	return ""
}
