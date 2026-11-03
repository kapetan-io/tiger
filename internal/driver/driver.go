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
// scheduling. The cross-package wave adds the finish step (ADR-0010): after
// every package has been visited, each registered finish function reads
// every fact its analyzer exported and reports on the whole module — the
// one place a whole-program rule can see both the declaring package and
// the packages that reference it.
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
	"github.com/kapetan-io/tiger/internal/finish"
	"github.com/kapetan-io/tiger/internal/layerfile"
	"github.com/kapetan-io/tiger/internal/worklist"
)

const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedTypes |
	packages.NeedTypesInfo |
	packages.NeedTypesSizes |
	packages.NeedSyntax |
	packages.NeedModule

type Finding struct {
	Position token.Position
	Category string
	Message  string
	Package  string
}

type Report struct {
	Findings []Finding
	Packages []string
}

func Check(
	root string,
	patterns []string,
	analyzers []*analysis.Analyzer,
	finishers []finish.Finisher,
) ([]Finding, error) {
	report, err := Run(root, patterns, analyzers, finishers)
	if err != nil {
		return nil, err
	}
	return report.Findings, nil
}

func Run(
	root string,
	patterns []string,
	analyzers []*analysis.Analyzer,
	finishers []finish.Finisher,
) (Report, error) {
	loaded, err := load(root, patterns)
	if err != nil {
		return Report{}, err
	}
	scheduled := analyzerOrder(analyzerClosure(analyzers))
	store := newFactsStore()
	findings := []Finding{}
	ordered := packageOrder(loaded)
	visited := []string{}
	for _, pkg := range ordered {
		visited = append(visited, pkg.PkgPath)
		run := &packageRun{
			pkg:       pkg,
			generated: generatedFiles(pkg),
			results:   map[*analysis.Analyzer]any{},
		}
		for _, pass := range scheduled {
			collected, err := runPass(pass, run, store)
			if err != nil {
				return Report{}, err
			}
			findings = append(findings, collected...)
		}
	}
	for _, finisher := range finishers {
		collected, err := runFinish(finisher, ordered, store)
		if err != nil {
			return Report{}, err
		}
		findings = append(findings, collected...)
	}
	relativize(findings, root)
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].Before(findings[j])
	})
	return Report{Findings: findings, Packages: visited}, nil
}

func analyzerClosure(requested []*analysis.Analyzer) []*analysis.Analyzer {
	seen := map[*analysis.Analyzer]bool{}
	all := []*analysis.Analyzer{}
	work := append([]*analysis.Analyzer{}, requested...)
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		next := work[i]
		if seen[next] {
			continue
		}
		seen[next] = true
		all = append(all, next)
		work = append(work, next.Requires...)
	}
	assert.Ok(i == len(work), "analyzerClosure: worklist reached worklist.NodesMax")
	return all
}

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

func packageOrder(loaded []*packages.Package) []*packages.Package {
	byID := map[string]*packages.Package{}
	ids := make([]string, 0, len(loaded))
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

func topological(keys []string, depsOf func(string) []string) []string {
	pending := append([]string{}, keys...)
	sort.Strings(pending)
	done := map[string]bool{}
	order := make([]string, 0, len(pending))
	for range keys {
		index := readyIndex(pending, done, depsOf)
		assert.Ok(index >= 0, "topological: dependency cycle")
		done[pending[index]] = true
		order = append(order, pending[index])
		pending = slices.Delete(pending, index, index+1)
	}
	return order
}

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

func dedupe(loaded []*packages.Package) []*packages.Package {
	augmented := map[string]bool{}
	for _, pkg := range loaded {
		plain, variant, found := strings.Cut(pkg.ID, " [")
		if found && variant == plain+".test]" {
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

func generatedFiles(pkg *packages.Package) map[string]bool {
	generated := map[string]bool{}
	for _, file := range pkg.Syntax {
		if ast.IsGenerated(file) {
			generated[pkg.Fset.Position(file.Package).Filename] = true
		}
	}
	return generated
}

type packageRun struct {
	pkg       *packages.Package
	generated map[string]bool
	results   map[*analysis.Analyzer]any
}

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
		Module:     moduleOf(run.pkg),
		ResultOf:   resultOf,
		Report: func(diagnostic analysis.Diagnostic) {
			position := run.pkg.Fset.Position(diagnostic.Pos)
			if run.generated[position.Filename] && diagnostic.Category != layerfile.RuleID {
				return
			}
			findings = append(findings, Finding{
				Position: position,
				Category: diagnostic.Category,
				Message:  diagnostic.Message,
				Package:  run.pkg.PkgPath,
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

func runFinish(
	finisher finish.Finisher,
	ordered []*packages.Package,
	store *factsStore,
) (findings []Finding, err error) {
	name := finisher.Analyzer.Name
	defer func() {
		if recovered := recover(); recovered != nil {
			findings = nil
			err = fmt.Errorf("finish step %s panicked: %v", name, recovered)
		}
	}()
	program := &finish.Program{
		Fset:         ordered[0].Fset,
		Packages:     make([]finish.Package, 0, len(ordered)),
		PackageFacts: store.allPackages(finisher.Analyzer)(),
		ObjectFacts:  store.allSymbols(finisher.Analyzer)(),
	}
	loadedFiles := map[string]bool{}
	generated := map[string]bool{}
	for _, pkg := range ordered {
		program.Packages = append(program.Packages, finish.Package{
			Pkg: pkg.Types, TypesInfo: pkg.TypesInfo, Syntax: pkg.Syntax,
		})
		for _, file := range pkg.CompiledGoFiles {
			loadedFiles[file] = true
		}
		maps.Copy(generated, generatedFiles(pkg))
	}
	diagnostics, runErr := finisher.Run(program)
	if runErr != nil {
		return nil, fmt.Errorf("finish step %s failed: %w", name, runErr)
	}
	for _, diagnostic := range diagnostics {
		position := program.Fset.Position(diagnostic.Pos)
		if !loadedFiles[position.Filename] {
			return nil, fmt.Errorf(
				"finish step %s reported a finding outside the loaded packages: %s",
				name, diagnostic.Message,
			)
		}
		if generated[position.Filename] {
			continue
		}
		findings = append(findings, Finding{
			Position: position,
			Category: diagnostic.Category,
			Message:  diagnostic.Message,
		})
	}
	return findings, nil
}

func moduleOf(pkg *packages.Package) *analysis.Module {
	if pkg.Module == nil {
		return nil
	}
	return &analysis.Module{
		Path: pkg.Module.Path, Version: pkg.Module.Version, GoVersion: pkg.Module.GoVersion,
	}
}

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

type symbolFactKey struct {
	analyzer *analysis.Analyzer
	factType reflect.Type
	pkgPath  string
	path     objectpath.Path
	identity types.Object
}

type packageFactKey struct {
	analyzer *analysis.Analyzer
	factType reflect.Type
	pkgPath  string
}

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

func registeredFactType(analyzer *analysis.Analyzer, fact analysis.Fact) bool {
	want := reflect.TypeOf(fact)
	for _, prototype := range analyzer.FactTypes {
		if reflect.TypeOf(prototype) == want {
			return true
		}
	}
	return false
}

func cloneFact(fact analysis.Fact) analysis.Fact {
	original := reflect.ValueOf(fact).Elem()
	clone := reflect.New(original.Type())
	clone.Elem().Set(original)
	copied, ok := clone.Interface().(analysis.Fact)
	assert.Ok(ok, "a cloned fact is a fact")
	return copied
}

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

func (f Finding) Before(other Finding) bool {
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
