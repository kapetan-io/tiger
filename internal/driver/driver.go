// Package driver loads Go packages and runs the registered analyzers over
// them, producing position-sorted findings.
//
// The driver owns no severity or exit-code policy — it reports what fired
// and where; the CLI applies the registry's severity (ADR-0002). What the
// driver does own is correctness constraint 5: a package that fails to load
// or an analyzer that fails to run is a terminal error for the whole run,
// never a silent shrink of the rule set. This wave adds two more driver
// responsibilities, both still in service of constraint 5: resolving
// analyzer `Requires` (buildssa and its own dependencies) and propagating
// `go/analysis` facts across packages in a stable, module-local order — so
// fact content, like findings, never depends on map iteration or
// scheduling.
package driver

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/objectpath"

	"github.com/kapetan-io/tiger/assert"
)

// loadMode requests what every analyzer needs, single-package or SSA:
// NeedDeps extends the dependency graph packages.Load reports (pkg.Imports
// entries carry full IDs, not stubs) so the driver's own topological sort
// and buildssa's construction of a package's direct-import SSA packages
// both have what they need.
const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedDeps |
	packages.NeedTypes |
	packages.NeedTypesInfo |
	packages.NeedTypesSizes |
	packages.NeedSyntax |
	packages.NeedModule

// Finding is one diagnostic, located and categorized. The category resolves
// to a registry entry; the message names the rule and the compliant form.
type Finding struct {
	Position token.Position
	Category string
	Message  string
}

// Check loads the packages matched by patterns under root and runs every
// analyzer — plus every analyzer transitively named in an analyzer's
// Requires — over each package. Packages are visited in a stable
// topological order (dependencies before dependents) so an analyzer's
// exported facts are available by the time a dependent package is
// analyzed; buildssa's SSA build runs once per package and is shared by
// every analyzer requiring it there. Findings come back sorted by
// position, with filenames relative to root so no absolute path reaches
// any output. A load error, a type error, or an analyzer failure returns
// an error: partial results are never presented as a complete run.
func Check(root string, patterns []string, analyzers []*analysis.Analyzer) ([]Finding, error) {
	loaded, err := load(root, patterns)
	if err != nil {
		return nil, err
	}
	scheduled := analyzerOrder(analyzerClosure(analyzers))
	store := newFactsStore()
	findings := []Finding{}
	for _, pkg := range packageOrder(loaded) {
		run := &packageRun{
			pkg:       pkg,
			generated: generatedFiles(pkg),
			results:   map[*analysis.Analyzer]any{},
		}
		for _, pass := range scheduled {
			collected, err := runPass(pass, run, store)
			if err != nil {
				return nil, err
			}
			findings = append(findings, collected...)
		}
	}
	relativize(findings, root)
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].before(findings[j])
	})
	return findings, nil
}

// analyzerClosure computes the transitive closure of requested over
// Requires, so a caller lists only the analyzers it wants and dependencies
// like buildssa (and buildssa's own dependency, ctrlflow) are pulled in
// automatically.
func analyzerClosure(requested []*analysis.Analyzer) []*analysis.Analyzer {
	seen := map[*analysis.Analyzer]bool{}
	all := []*analysis.Analyzer{}
	work := append([]*analysis.Analyzer{}, requested...)
	for i := 0; i < len(work); i++ {
		next := work[i]
		if seen[next] {
			continue
		}
		seen[next] = true
		all = append(all, next)
		work = append(work, next.Requires...)
	}
	return all
}

// analyzerOrder returns closure in dependency-first order: every
// analyzer's Requires precede it, tie-broken lexicographically on name so
// the run order — and therefore the order facts and results become
// available — never depends on the slice order the caller passed in.
func analyzerOrder(closure []*analysis.Analyzer) []*analysis.Analyzer {
	byName := map[string]*analysis.Analyzer{}
	names := make([]string, 0, len(closure))
	for _, a := range closure {
		byName[a.Name] = a
		names = append(names, a.Name)
	}
	depsOf := func(name string) []string {
		requires := byName[name].Requires
		deps := make([]string, 0, len(requires))
		for _, req := range requires {
			deps = append(deps, req.Name)
		}
		return deps
	}
	ordered := make([]*analysis.Analyzer, 0, len(names))
	for _, name := range topological(names, depsOf) {
		ordered = append(ordered, byName[name])
	}
	return ordered
}

// packageOrder returns loaded in a stable topological order: every
// package's loaded-set imports precede it, tie-broken lexicographically on
// pkg.ID. This is restricted to the loaded set on purpose — a dependency
// the caller's patterns did not match is out of scope for facts this run
// (a marked known-miss, per the blueprint), never a driver-internal load.
func packageOrder(loaded []*packages.Package) []*packages.Package {
	byID := map[string]*packages.Package{}
	ids := make([]string, 0, len(loaded))
	// An import edge names the plain variant ("foo") even when dedupe kept
	// only the test-augmented one ("foo [foo.test]"); variantOf remaps the
	// edge so the dependency still precedes its importer.
	variantOf := map[string]string{}
	for _, pkg := range loaded {
		byID[pkg.ID] = pkg
		ids = append(ids, pkg.ID)
		if plain, _, found := strings.Cut(pkg.ID, " ["); found {
			variantOf[plain] = pkg.ID
		}
	}
	depsOf := func(id string) []string {
		imports := byID[id].Imports
		deps := []string{}
		for _, path := range slices.Sorted(maps.Keys(imports)) {
			imported := imports[path]
			target := imported.ID
			if _, inSet := byID[target]; !inSet {
				target = variantOf[target]
			}
			if _, inSet := byID[target]; inSet {
				deps = append(deps, target)
			}
		}
		return deps
	}
	ordered := make([]*packages.Package, 0, len(ids))
	for _, id := range topological(ids, depsOf) {
		ordered = append(ordered, byID[id])
	}
	return ordered
}

// topological orders keys so every key named by depsOf precedes the key
// that depends on it, breaking ties lexicographically so the result never
// depends on map iteration or scheduling (constraint 5). The worklist only
// shrinks: each pass picks the lexicographically first ready key out of
// what remains and removes it.
func topological(keys []string, depsOf func(string) []string) []string {
	pending := append([]string{}, keys...)
	sort.Strings(pending)
	done := map[string]bool{}
	order := make([]string, 0, len(pending))
	// Each pass removes exactly one ready key, so the walk takes exactly
	// len(keys) passes — a structural bound, no separate counter needed.
	for range keys {
		index := readyIndex(pending, done, depsOf)
		assert.Ok(index >= 0, "topological: dependency cycle")
		done[pending[index]] = true
		order = append(order, pending[index])
		pending = slices.Delete(pending, index, index+1)
	}
	return order
}

// readyIndex returns the position in pending (already sorted) of the first
// key whose dependencies are all done, or -1 if none is ready — which can
// only happen on a cycle, since an acyclic graph always has a ready node.
func readyIndex(pending []string, done map[string]bool, depsOf func(string) []string) int {
	for i, key := range pending {
		ready := true
		for _, dep := range depsOf(key) {
			if !done[dep] {
				ready = false
				break
			}
		}
		if ready {
			return i
		}
	}
	return -1
}

// load returns the packages to analyze: every matched package including its
// test files, deduplicated so no file is analyzed twice.
func load(root string, patterns []string) ([]*packages.Package, error) {
	config := &packages.Config{
		Mode:  loadMode,
		Dir:   root,
		Tests: true,
	}
	loaded, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	if len(loaded) == 0 {
		return nil, errors.New("no packages matched")
	}
	kept := dedupe(loaded)
	failures := []string{}
	for _, pkg := range kept {
		for _, loadErr := range pkg.Errors {
			failures = append(failures, loadErr.Error())
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		return nil, errors.New("packages failed to load:\n" + strings.Join(failures, "\n"))
	}
	return kept, nil
}

// dedupe drops the synthesized test main and, when a test-augmented variant
// of a package is present, the plain variant it duplicates.
func dedupe(loaded []*packages.Package) []*packages.Package {
	augmented := map[string]bool{}
	for _, pkg := range loaded {
		plain, _, found := strings.Cut(pkg.ID, " [")
		if found && !strings.HasSuffix(plain, "_test") {
			augmented[plain] = true
		}
	}
	kept := []*packages.Package{}
	for _, pkg := range loaded {
		if strings.HasSuffix(pkg.ID, ".test") {
			continue
		}
		if augmented[pkg.ID] {
			continue
		}
		kept = append(kept, pkg)
	}
	return kept
}

// generatedFiles names the package's machine-generated sources, per the
// standard "// Code generated ... DO NOT EDIT." convention. The files still
// load and type-check — hand-written code that depends on them is analyzed
// in full — but a finding inside one names nothing a person can fix, so the
// driver drops it there.
func generatedFiles(pkg *packages.Package) map[string]bool {
	generated := map[string]bool{}
	for _, file := range pkg.Syntax {
		if ast.IsGenerated(file) {
			generated[pkg.Fset.Position(file.Package).Filename] = true
		}
	}
	return generated
}

// packageRun bundles what one package's analyzer loop shares: the loaded
// package, its generated-file set, and the per-analyzer result cache that
// gives buildssa (and any other required analyzer) its "computed once per
// package, shared by every dependent" contract.
type packageRun struct {
	pkg       *packages.Package
	generated map[string]bool
	results   map[*analysis.Analyzer]any
}

// runPass runs one analyzer over one package, collecting its diagnostics
// outside the generated files, plumbing the results of its own Requires
// from run's cache, and wiring facts through store. A panic inside the
// analyzer is converted to an error so the run can exit with an
// operational failure instead of presenting partial results.
func runPass(
	pass *analysis.Analyzer,
	run *packageRun,
	store *factsStore,
) (findings []Finding, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			findings = nil
			err = fmt.Errorf(
				"analyzer %s panicked on %s: %v", pass.Name, run.pkg.PkgPath, recovered,
			)
		}
	}()
	resultOf := map[*analysis.Analyzer]any{}
	for _, req := range pass.Requires {
		resultOf[req] = run.results[req]
	}
	unit := &analysis.Pass{
		Analyzer:   pass,
		Fset:       run.pkg.Fset,
		Files:      run.pkg.Syntax,
		OtherFiles: run.pkg.OtherFiles,
		Pkg:        run.pkg.Types,
		TypesInfo:  run.pkg.TypesInfo,
		TypesSizes: run.pkg.TypesSizes,
		ResultOf:   resultOf,
		Report: func(diagnostic analysis.Diagnostic) {
			position := run.pkg.Fset.Position(diagnostic.Pos)
			if run.generated[position.Filename] {
				return
			}
			findings = append(findings, Finding{
				Position: position,
				Category: diagnostic.Category,
				Message:  diagnostic.Message,
			})
		},
		ImportObjectFact:  store.importSymbol(pass),
		ExportObjectFact:  store.exportSymbol(pass, run.pkg.Types),
		ImportPackageFact: store.importPackage(pass),
		ExportPackageFact: store.exportPackage(pass, run.pkg.Types),
		AllObjectFacts:    store.allSymbols(pass),
		AllPackageFacts:   store.allPackages(pass),
	}
	result, runErr := pass.Run(unit)
	if runErr != nil {
		return nil, fmt.Errorf("analyzer %s failed on %s: %w", pass.Name, run.pkg.PkgPath, runErr)
	}
	run.results[pass] = result
	return findings, nil
}

// factsStore is the driver's in-memory, module-local implementation of the
// go/analysis facts mechanism — never serialized, since facts exist only
// for the duration of one Check call. One store spans the whole call, so a
// fact exported while analyzing a dependency package is visible while
// analyzing any package the topological order visits afterward.
//
// Facts on a types.Object (a name's compiler-resolved meaning: a func, a
// var, a field, and so on) are keyed by (analyzer, fact type, the named
// entity). A
// types.Object pointer is canonical only within one packages.Load call, and
// packages.Load produces distinct type-checked variants for a package
// under test ("pkg" vs "pkg [pkg.test]") that duplicate the same source
// entities under different pointers. Exported entities are therefore keyed
// by their stable (package path, objectpath.Path) instead, which survives
// that seam; unexported entities have no such path and fall back to
// identity, which is the best available key for them.
type factsStore struct {
	symbols      map[symbolFactKey]analysis.Fact
	symbolOrder  []symbolFactKey
	packages     map[packageFactKey]analysis.Fact
	packageOrder []packageFactKey
	typePackages map[string]*types.Package
}

func newFactsStore() *factsStore {
	return &factsStore{
		symbols:      map[symbolFactKey]analysis.Fact{},
		packages:     map[packageFactKey]analysis.Fact{},
		typePackages: map[string]*types.Package{},
	}
}

// symbolFactKey identifies one (analyzer, fact type, types.Object) triple.
// The path field is set for exported entities (the cross-seam-stable key);
// the identity field is set otherwise (the fallback for unexported
// entities).
type symbolFactKey struct {
	analyzer *analysis.Analyzer
	factType reflect.Type
	pkgPath  string
	path     objectpath.Path
	identity types.Object
}

// packageFactKey identifies one (analyzer, fact type, package) triple. A
// package's import path is already stable across the test-variant seam, so
// the identity fallback problem above does not apply here.
type packageFactKey struct {
	analyzer *analysis.Analyzer
	factType reflect.Type
	pkgPath  string
}

// symbolKey computes entity's key for analyzer's fact type, preferring the
// objectpath-based form whenever entity is exported and has one.
func symbolKey(analyzer *analysis.Analyzer, entity types.Object, fact analysis.Fact) symbolFactKey {
	factType := reflect.TypeOf(fact)
	if entity.Exported() {
		if path, err := objectpath.For(entity); err == nil {
			return symbolFactKey{
				analyzer: analyzer, factType: factType, pkgPath: entity.Pkg().Path(), path: path,
			}
		}
	}
	return symbolFactKey{analyzer: analyzer, factType: factType, identity: entity}
}

// registeredFactType reports whether fact's concrete type is one analyzer
// declared in FactTypes. Exporting an undeclared fact type is a
// programming error, not an operating condition, so the driver asserts
// rather than returning it as a run error.
func registeredFactType(analyzer *analysis.Analyzer, fact analysis.Fact) bool {
	want := reflect.TypeOf(fact)
	for _, prototype := range analyzer.FactTypes {
		if reflect.TypeOf(prototype) == want {
			return true
		}
	}
	return false
}

// cloneFact copies fact's pointee into a new allocation, so a caller
// mutating its own value after exporting it can never retroactively change
// what the store hands back to a later importer.
func cloneFact(fact analysis.Fact) analysis.Fact {
	original := reflect.ValueOf(fact).Elem()
	clone := reflect.New(original.Type())
	clone.Elem().Set(original)
	copied, ok := clone.Interface().(analysis.Fact)
	assert.Ok(ok, "a cloned fact is a fact")
	return copied
}

// exportSymbol returns the ExportObjectFact closure for one analyzer
// analyzing one package.
func (s *factsStore) exportSymbol(
	analyzer *analysis.Analyzer, pkg *types.Package,
) func(types.Object, analysis.Fact) {
	return func(entity types.Object, fact analysis.Fact) {
		assert.Ok(
			entity.Pkg() == pkg,
			"ExportObjectFact called on an entity outside the analyzed package",
		)
		assert.Ok(
			registeredFactType(analyzer, fact), "exported fact's type is not declared in FactTypes",
		)
		s.typePackages[pkg.Path()] = pkg
		key := symbolKey(analyzer, entity, fact)
		if _, exists := s.symbols[key]; !exists {
			s.symbolOrder = append(s.symbolOrder, key)
		}
		s.symbols[key] = cloneFact(fact)
	}
}

// importSymbol returns the ImportObjectFact closure for one analyzer.
func (s *factsStore) importSymbol(
	analyzer *analysis.Analyzer,
) func(types.Object, analysis.Fact) bool {
	return func(entity types.Object, fact analysis.Fact) bool {
		stored, found := s.symbols[symbolKey(analyzer, entity, fact)]
		if !found {
			return false
		}
		reflect.ValueOf(fact).Elem().Set(reflect.ValueOf(stored).Elem())
		return true
	}
}

// exportPackage returns the ExportPackageFact closure for one analyzer
// analyzing one package.
func (s *factsStore) exportPackage(
	analyzer *analysis.Analyzer, pkg *types.Package,
) func(analysis.Fact) {
	return func(fact analysis.Fact) {
		assert.Ok(
			registeredFactType(analyzer, fact), "exported fact's type is not declared in FactTypes",
		)
		s.typePackages[pkg.Path()] = pkg
		key := packageFactKey{
			analyzer: analyzer, factType: reflect.TypeOf(fact), pkgPath: pkg.Path(),
		}
		if _, exists := s.packages[key]; !exists {
			s.packageOrder = append(s.packageOrder, key)
		}
		s.packages[key] = cloneFact(fact)
	}
}

// importPackage returns the ImportPackageFact closure for one analyzer.
func (s *factsStore) importPackage(
	analyzer *analysis.Analyzer,
) func(*types.Package, analysis.Fact) bool {
	return func(pkg *types.Package, fact analysis.Fact) bool {
		key := packageFactKey{
			analyzer: analyzer, factType: reflect.TypeOf(fact), pkgPath: pkg.Path(),
		}
		stored, found := s.packages[key]
		if !found {
			return false
		}
		reflect.ValueOf(fact).Elem().Set(reflect.ValueOf(stored).Elem())
		return true
	}
}

// allSymbols returns the AllObjectFacts closure for one analyzer. It walks
// symbolOrder — a slice recording export order — rather than ranging the
// map directly, so enumeration is deterministic without depending on map
// iteration order.
func (s *factsStore) allSymbols(analyzer *analysis.Analyzer) func() []analysis.ObjectFact {
	return func() []analysis.ObjectFact {
		facts := []analysis.ObjectFact{}
		for _, key := range s.symbolOrder {
			if key.analyzer != analyzer {
				continue
			}
			entity := s.resolveSymbol(key)
			if entity == nil {
				continue
			}
			facts = append(facts, analysis.ObjectFact{Object: entity, Fact: s.symbols[key]})
		}
		return facts
	}
}

// allPackages returns the AllPackageFacts closure for one analyzer,
// walking packageOrder for the same determinism reason as allSymbols.
func (s *factsStore) allPackages(analyzer *analysis.Analyzer) func() []analysis.PackageFact {
	return func() []analysis.PackageFact {
		facts := []analysis.PackageFact{}
		for _, key := range s.packageOrder {
			if key.analyzer != analyzer {
				continue
			}
			pkg, found := s.typePackages[key.pkgPath]
			if !found {
				continue
			}
			facts = append(facts, analysis.PackageFact{Package: pkg, Fact: s.packages[key]})
		}
		return facts
	}
}

// resolveSymbol recovers the types.Object a key names: the identity
// fallback stores it directly; the path form resolves it through the
// package registered at export time.
func (s *factsStore) resolveSymbol(key symbolFactKey) types.Object {
	if key.path == "" {
		return key.identity
	}
	pkg, found := s.typePackages[key.pkgPath]
	if !found {
		return nil
	}
	entity, err := objectpath.Object(pkg, key.path)
	if err != nil {
		return nil
	}
	return entity
}

// relativize rewrites every finding's filename relative to root, keeping
// absolute paths out of the output (constraint 4).
func relativize(findings []Finding, root string) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return
	}
	for i := range findings {
		relative, err := filepath.Rel(absRoot, findings[i].Position.Filename)
		if err == nil && !strings.HasPrefix(relative, "..") {
			findings[i].Position.Filename = filepath.ToSlash(relative)
		}
	}
}

// before orders findings by position, then category and message so equal
// positions still sort deterministically.
func (f Finding) before(other Finding) bool {
	if f.Position.Filename != other.Position.Filename {
		return f.Position.Filename < other.Position.Filename
	}
	if f.Position.Line != other.Position.Line {
		return f.Position.Line < other.Position.Line
	}
	if f.Position.Column != other.Position.Column {
		return f.Position.Column < other.Position.Column
	}
	if f.Category != other.Category {
		return f.Category < other.Category
	}
	return f.Message < other.Message
}
