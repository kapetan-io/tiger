// Package driver loads Go packages and runs the registered analyzers over
// them, producing position-sorted findings.
//
// The driver owns no severity or exit-code policy — it reports what fired
// and where; the CLI applies the registry's severity (ADR-0002). What the
// driver does own is correctness constraint 5: a package that fails to load
// or an analyzer that fails to run is a terminal error for the whole run,
// never a silent shrink of the rule set.
package driver

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/kapetan-io/tiger/assert"
)

// loadMode requests everything a single-package AST-and-types analyzer
// needs; wave 1 needs no SSA and no cross-package facts.
const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
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
// analyzer over each. Findings come back sorted by position, with filenames
// relative to root so no absolute path reaches any output. A load error, a
// type error, or an analyzer failure returns an error: partial results are
// never presented as a complete run.
func Check(root string, patterns []string, analyzers []*analysis.Analyzer) ([]Finding, error) {
	loaded, err := load(root, patterns)
	if err != nil {
		return nil, err
	}
	findings := []Finding{}
	for _, pkg := range loaded {
		generated := generatedFiles(pkg)
		for _, pass := range analyzers {
			collected, err := runPass(pass, pkg, generated)
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

// runPass runs one analyzer over one package, collecting its diagnostics
// outside the generated files. A panic inside the analyzer is converted to
// an error so the run can exit with an operational failure instead of
// presenting partial results.
func runPass(
	pass *analysis.Analyzer,
	pkg *packages.Package,
	generated map[string]bool,
) (findings []Finding, err error) {
	// Wave-1 analyzers are pure single-package passes; the driver does not
	// plumb dependency results or facts, and registration of an analyzer
	// that needs them is a programming error.
	assert.Ok(len(pass.Requires) == 0, "wave-1 analyzers declare no Requires")
	assert.Ok(len(pass.FactTypes) == 0, "wave-1 analyzers declare no facts")

	defer func() {
		if recovered := recover(); recovered != nil {
			findings = nil
			err = fmt.Errorf("analyzer %s panicked on %s: %v", pass.Name, pkg.PkgPath, recovered)
		}
	}()
	unit := &analysis.Pass{
		Analyzer:   pass,
		Fset:       pkg.Fset,
		Files:      pkg.Syntax,
		OtherFiles: pkg.OtherFiles,
		Pkg:        pkg.Types,
		TypesInfo:  pkg.TypesInfo,
		TypesSizes: pkg.TypesSizes,
		ResultOf:   map[*analysis.Analyzer]any{},
		Report: func(diagnostic analysis.Diagnostic) {
			position := pkg.Fset.Position(diagnostic.Pos)
			if generated[position.Filename] {
				return
			}
			findings = append(findings, Finding{
				Position: position,
				Category: diagnostic.Category,
				Message:  diagnostic.Message,
			})
		},
	}
	if _, err := pass.Run(unit); err != nil {
		return nil, fmt.Errorf("analyzer %s failed on %s: %w", pass.Name, pkg.PkgPath, err)
	}
	return findings, nil
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
