// Package restrictions enforces TS-P01 and TS-P02, both blocking: a
// package's declared restriction axes hold against its own file set, and
// every transitively imported module package supports each axis the
// package claims — a claim a dependency does not back is a contract the
// code contradicts (ADR-0012), and the finding names the edit.
//
// TS-P01 needs nothing beyond the package's own imports and doc comment.
// TS-P02 needs every transitively imported package's own declaration, so
// each package exports its parsed declaration as a fact and TS-P02 is
// computed forward from the facts of packages already visited — a
// per-package rule, unlike the three whole-program rules this wave adds.
package restrictions

import (
	"go/types"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/restrict"
	"github.com/kapetan-io/tiger/internal/directive"
)

// Fact carries a package's own declared restriction across the facts
// mechanism, so a dependent package's TS-P02 bound can read what this
// package claimed without re-parsing its source.
type Fact struct {
	Restriction directive.Restriction
}

// AFact marks Fact as a go/analysis fact.
func (*Fact) AFact() {}

// String renders a fact in the same //tiger:restrict text FormatRestrict
// prints, so analysistest's fact expectations read the same vocabulary as
// everything else this analyzer prints.
func (f *Fact) String() string {
	return directive.FormatRestrict(f.Restriction)
}

// Analyzer enforces TS-P01 and TS-P02.
var Analyzer = &analysis.Analyzer{
	Name: "restrictions",
	Doc: "TS-P01 and TS-P02: declared package restrictions hold, and every dependency " +
		"supports each claimed axis.",
	FactTypes: []analysis.Fact{new(Fact)},
	Run:       run,
}

func run(pass *analysis.Pass) (any, error) {
	declarations := restrict.Collect(pass)
	if len(declarations) == 0 {
		return nil, nil
	}
	if len(declarations) > 1 {
		pass.Report(analysis.Diagnostic{
			Pos:      declarations[1].Pos,
			Category: "TS-P01",
			Message: "TS-P01: package " + pass.Pkg.Name() + " has two //tiger:restrict " +
				"directives — keep one, merging the axes into a single comma-separated list",
		})
	}
	declared := declarations[0]
	pass.ExportPackageFact(&Fact{Restriction: declared.Restriction})

	checkNoReflect(pass, declared.Restriction)
	checkImports(pass, declared.Restriction)
	reportPrecisionBound(pass, declared)
	return nil, nil
}

// checkNoReflect reports one TS-P01 per import spec naming "reflect" when
// the declaration claims no-reflect.
func checkNoReflect(pass *analysis.Pass, declared directive.Restriction) {
	if !declared.NoReflect {
		return
	}
	for _, file := range pass.Files {
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || path != "reflect" {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:      spec.Pos(),
				Category: "TS-P01",
				Message: "TS-P01: package " + pass.Pkg.Name() + " declares " +
					"//tiger:restrict no-reflect but imports reflect — remove the reflect " +
					"import or drop no-reflect from the directive",
			})
		}
	}
}

// checkImports reports one TS-P01 per import spec outside the declared
// import list, when the declaration claims that axis. Stdlib is always
// allowed, per the TS-D01 baseline the axis extends.
func checkImports(pass *analysis.Pass, declared directive.Restriction) {
	if len(declared.Imports) == 0 {
		return
	}
	module := modulePath(pass)
	for _, file := range pass.Files {
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || isStdlib(path) || allowedImport(path, declared.Imports, module) {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:      spec.Pos(),
				Category: "TS-P01",
				Message: "TS-P01: package " + pass.Pkg.Name() + " imports " + path +
					", which its //tiger:restrict imports(...) list does not allow — remove " +
					"the import or add the path to imports(...)",
			})
		}
	}
}

// modulePath returns the loaded module's path, or "" when the driver has
// none to report (analysistest's synthetic module).
func modulePath(pass *analysis.Pass) string {
	if pass.Module == nil {
		return ""
	}
	return pass.Module.Path
}

// allowedImport reports whether path matches one of patterns, each
// resolved relative to module: a bare pattern matches exactly, a
// pattern ending in the subtree suffix matches its whole subtree.
func allowedImport(path string, patterns []string, module string) bool {
	for _, pattern := range patterns {
		full := pattern
		if module != "" {
			full = module + "/" + pattern
		}
		if prefix, ok := strings.CutSuffix(full, directive.Subtree); ok {
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return true
			}
			continue
		}
		if path == full {
			return true
		}
	}
	return false
}

// isStdlib reports whether path is a standard-library import: its first
// segment carries no dot, the signal every module path outside the
// standard library carries (a domain name).
func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// axis names one restriction dimension, in TS-P02's reporting order.
type axis struct {
	name    string
	claimed func(directive.Restriction) bool
}

var axes = []axis{
	{"closed-dispatch", func(r directive.Restriction) bool { return r.ClosedDispatch }},
	{"no-reflect", func(r directive.Restriction) bool { return r.NoReflect }},
	{"imports(...)", func(r directive.Restriction) bool { return len(r.Imports) > 0 }},
}

// reportPrecisionBound emits one TS-P02 finding per axis this package
// claims but that some transitively imported module package weakens: an
// axis the dependency does not claim (declares nothing) or explicitly
// leaves unclaimed (declares other axes but not this one).
func reportPrecisionBound(pass *analysis.Pass, declared restrict.Declaration) {
	weakestBy := weakestDependency(pass)
	for _, one := range axes {
		if !one.claimed(declared.Restriction) {
			continue
		}
		dep, found := weakestBy[one.name]
		if !found {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      declared.File.Package,
			Category: "TS-P02",
			Message:  precisionMessage(pass.Pkg.Name(), dep, one.name),
		})
	}
}

// weakener names one dependency that leaves an axis unclaimed: its
// display path, and whether it declared no //tiger:restrict at all
// ("declares nothing") versus declaring other axes without this one
// ("does not claim <axis>").
type weakener struct {
	path       string
	undeclared bool
}

// precisionMessage renders one TS-P02 finding for the given axis, naming
// dep as the dependency that leaves it unclaimed and the two edits that
// resolve it. The axis name is already the exact //tiger:restrict spelling
// of the axis, so it doubles as the claim text.
func precisionMessage(pkgName string, dep weakener, axisName string) string {
	reason := "does not claim " + axisName
	if dep.undeclared {
		reason = "declares nothing"
	}
	return "TS-P02: package " + pkgName + " claims " + axisName + " but imports " + dep.path +
		", which " + reason + " — add //tiger:restrict " + axisName + " to " + dep.path +
		", or drop " + axisName + " from " + pkgName + "'s declaration"
}

// weakestDependency finds, for each axis this analyzer tracks, the
// lexicographically first transitively imported module package that
// leaves the axis unclaimed — either by declaring nothing at all or by
// declaring other axes without this one. A third-party import (outside the
// loaded module, or the module is unknown) is weakest on every axis, per
// the specification's honesty rule, and is named by its full import path
// since it has no module-relative form.
func weakestDependency(pass *analysis.Pass) map[string]weakener {
	display := displayPath(modulePath(pass))
	weakest := map[string]weakener{}
	for _, dep := range reachableModulePackages(pass) {
		declaration, hasFact := importedFact(pass, dep)
		found := weakener{path: display(dep.Path()), undeclared: !hasFact}
		for _, one := range axes {
			if _, already := weakest[one.name]; already {
				continue
			}
			if !hasFact || !one.claimed(declaration) {
				weakest[one.name] = found
			}
		}
	}
	return weakest
}

// importedFact reads target's own restriction fact, if it exported one.
func importedFact(pass *analysis.Pass, target *types.Package) (directive.Restriction, bool) {
	fact := new(Fact)
	if !pass.ImportPackageFact(target, fact) {
		return directive.Restriction{}, false
	}
	return fact.Restriction, true
}

// displayPath returns a renderer that shows a dependency path relative to
// module when it is inside the module, or as its full import path
// otherwise (third-party) — a closure so the module prefix is bound once
// per package, not threaded through every path it renders.
func displayPath(module string) func(path string) string {
	return func(path string) string {
		if module == "" {
			return path
		}
		if relative, ok := strings.CutPrefix(path, module+"/"); ok {
			return relative
		}
		return path
	}
}

// reachableModulePackages returns, sorted by import path, every non-stdlib
// package transitively imported by pass.Pkg — the set TS-P02's bound
// ranges over. The standard library is ground and never weakens a bound.
func reachableModulePackages(pass *analysis.Pass) []*types.Package {
	seen := map[string]bool{pass.Pkg.Path(): true}
	work := allImports(pass.Pkg.Imports())
	reached := []*types.Package{}
	for i := 0; i < len(work); i++ {
		pkg := work[i]
		if seen[pkg.Path()] {
			continue
		}
		seen[pkg.Path()] = true
		if isStdlib(pkg.Path()) {
			continue
		}
		reached = append(reached, pkg)
		work = append(work, allImports(pkg.Imports())...)
	}
	sort.Slice(reached, func(i, j int) bool { return reached[i].Path() < reached[j].Path() })
	return reached
}

// allImports returns imports as a plain slice, sorted by path, so the
// worklist order this package builds never depends on types.Package's own
// slice order (which is already deterministic, but sorting here keeps the
// dependency explicit rather than borrowed).
func allImports(imports []*types.Package) []*types.Package {
	sorted := append([]*types.Package{}, imports...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path() < sorted[j].Path() })
	return sorted
}
