// Package invariants is the per-package half shared by invariantrefs
// (TS-A07) and invariantnegative (TS-A09), plus the finish-step counting
// both rules apply to the whole module.
//
// An invariant const is a package-level const whose declared type is a
// named string type that some assert.Invariant or assert.Violates call in
// the module takes as its ID — defined by use, not by living in a package
// named inv. The assert package is recognized by shape (package name
// assert, function name Invariant or Violates), because adopters copy it
// into their own tree. Each package contributes what it declares and what
// it calls; only the finish step, seeing every contribution, can tell
// which consts are invariants and which of those go undefended.
package invariants

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/finish"
)

// Kind names which assert call referenced an invariant.
type Kind string

// FileClass says which files a call site or a search is confined to.
type FileClass bool

const (
	// Production is the non-_test.go half of a package: TS-A07's ground.
	Production FileClass = false
	// Tests is the _test.go half: TS-A09's ground.
	Tests FileClass = true
)

const (
	// KindAssert is an assert.Invariant call: the production-side defense
	// TS-A07 counts.
	KindAssert Kind = "assert"
	// KindViolates is an assert.Violates call: the negative-space proof
	// TS-A09 counts.
	KindViolates Kind = "violates"
)

// Const identifies one package-level const of a named string type,
// declared in a non-test file, by its package path and name — a key that
// survives the test-variant seam and gob encoding alike.
type Const struct {
	Pkg      string
	Name     string
	TypePkg  string
	TypeName string
}

// Call is one assert.Invariant or assert.Violates call: the named type of
// its ID argument (which makes that type's consts invariants), the
// package-level const it names when the argument is one, the enclosing
// function, and whether the call sits in a _test.go file.
type Call struct {
	Kind      Kind
	TypePkg   string
	TypeName  string
	ConstPkg  string
	ConstName string
	Function  string
	In        FileClass
}

// Contribution is what one package adds to the module-wide picture.
type Contribution struct {
	Consts []Const
	Calls  []Call
}

// Collect computes the package's contribution.
func Collect(pass *analysis.Pass) Contribution {
	contribution := Contribution{Consts: []Const{}, Calls: []Call{}}
	for _, file := range pass.Files {
		class := Production
		if strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go") {
			class = Tests
		}
		for _, decl := range file.Decls {
			switch typed := decl.(type) {
			case *ast.GenDecl:
				if typed.Tok == token.CONST && class == Production {
					contribution.Consts = append(contribution.Consts, consts(pass, typed)...)
				}
				contribution.Calls = append(contribution.Calls, calls(pass, site{
					node: typed, function: "", class: class,
				})...)
			case *ast.FuncDecl:
				contribution.Calls = append(contribution.Calls, calls(pass, site{
					node: typed, function: typed.Name.Name, class: class,
				})...)
			}
		}
	}
	return contribution
}

// consts returns the named-string-typed consts one const declaration
// declares.
func consts(pass *analysis.Pass, decl *ast.GenDecl) []Const {
	found := []Const{}
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range value.Names {
			object, ok := pass.TypesInfo.Defs[name].(*types.Const)
			if !ok {
				continue
			}
			named, ok := namedString(object.Type())
			if !ok {
				continue
			}
			found = append(found, Const{
				Pkg:      pass.Pkg.Path(),
				Name:     name.Name,
				TypePkg:  named.Obj().Pkg().Path(),
				TypeName: named.Obj().Name(),
			})
		}
	}
	return found
}

// site is one top-level declaration to search for assert calls: the
// function they attribute to (empty outside a function) and the file
// class they sit in.
type site struct {
	node     ast.Node
	function string
	class    FileClass
}

// calls returns every assert.Invariant and assert.Violates call under the
// site's node, attributed to its function (closures fold into their
// enclosing declaration).
func calls(pass *analysis.Pass, at site) []Call {
	found := []Call{}
	ast.Inspect(at.node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		kind, ok := assertKind(pass, call)
		if !ok {
			return true
		}
		named, ok := namedString(pass.TypesInfo.TypeOf(call.Args[0]))
		if !ok {
			return true
		}
		recorded := Call{
			Kind:     kind,
			TypePkg:  named.Obj().Pkg().Path(),
			TypeName: named.Obj().Name(),
			Function: at.function,
			In:       at.class,
		}
		if constant, ok := packageConst(pass, call.Args[0]); ok {
			recorded.ConstPkg = constant.Pkg().Path()
			recorded.ConstName = constant.Name()
		}
		found = append(found, recorded)
		return true
	})
	return found
}

// assertKind recognizes a call to assert.Invariant or assert.Violates by
// shape: a selector on a package named assert.
func assertKind(pass *analysis.Pass, call *ast.CallExpr) (Kind, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	callee, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok || callee.Pkg() == nil || callee.Pkg().Name() != "assert" {
		return "", false
	}
	switch callee.Name() {
	case "Invariant":
		return KindAssert, true
	case "Violates":
		return KindViolates, true
	}
	return "", false
}

// packageConst resolves expr to the package-level const it names, if it is
// a bare identifier or a qualified selector for one, seen through any
// parentheses.
func packageConst(pass *analysis.Pass, expr ast.Expr) (*types.Const, bool) {
	var ident *ast.Ident
	switch typed := ast.Unparen(expr).(type) {
	case *ast.Ident:
		ident = typed
	case *ast.SelectorExpr:
		ident = typed.Sel
	default:
		return nil, false
	}
	constant, ok := pass.TypesInfo.Uses[ident].(*types.Const)
	if !ok || constant.Pkg() == nil || constant.Parent() != constant.Pkg().Scope() {
		return nil, false
	}
	return constant, true
}

// namedString reports whether t is a named type with underlying string.
func namedString(t types.Type) (*types.Named, bool) {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return nil, false
	}
	basic, ok := named.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsString == 0 {
		return nil, false
	}
	return named, true
}

// Defense is what one rule demands of every invariant: a call of one kind
// in one file class.
type Defense struct {
	Kind Kind
	In   FileClass
}

// Undefended returns every invariant const in program that no call
// matching the demanded defense names — sorted by package path then name,
// so the caller's output never depends on fact order. Each result carries
// the const's resolved object, positioned in the loaded module.
func Undefended(
	program *finish.Program, contributions []Contribution, demanded Defense,
) []*types.Const {
	invariantTypes := map[Const]bool{}
	defended := map[Const]bool{}
	for _, contribution := range contributions {
		for _, call := range contribution.Calls {
			invariantTypes[Const{TypePkg: call.TypePkg, TypeName: call.TypeName}] = true
			if call.Kind != demanded.Kind || call.In != demanded.In || call.ConstName == "" {
				continue
			}
			if demanded.Kind == KindAssert && call.Function == "" {
				continue
			}
			defended[Const{Pkg: call.ConstPkg, Name: call.ConstName}] = true
		}
	}
	declared := []Const{}
	for _, contribution := range contributions {
		for _, constant := range contribution.Consts {
			if !invariantTypes[Const{TypePkg: constant.TypePkg, TypeName: constant.TypeName}] {
				continue
			}
			if defended[Const{Pkg: constant.Pkg, Name: constant.Name}] {
				continue
			}
			declared = append(declared, constant)
		}
	}
	sort.Slice(declared, func(i, j int) bool {
		if declared[i].Pkg != declared[j].Pkg {
			return declared[i].Pkg < declared[j].Pkg
		}
		return declared[i].Name < declared[j].Name
	})
	resolved := []*types.Const{}
	for _, constant := range declared {
		pkg, found := program.Lookup(constant.Pkg)
		if !found {
			continue
		}
		object, ok := pkg.Pkg.Scope().Lookup(constant.Name).(*types.Const)
		if !ok {
			continue
		}
		resolved = append(resolved, object)
	}
	return resolved
}

// Qualified names a const the way source does: package name dot name.
func Qualified(constant *types.Const) string {
	return constant.Pkg().Name() + "." + constant.Name()
}
