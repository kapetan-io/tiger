package cli

import (
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kapetan-io/tiger/internal/analyzers/testpackage"
	"github.com/kapetan-io/tiger/internal/config"
	"github.com/kapetan-io/tiger/internal/driver"
)

// testPackageCoverage returns the TS-T12 findings for test files the run did
// not load or report: a file a build constraint leaves out on this platform,
// and a generated file, whose findings the driver drops. It reads only the
// package clause of each Go file in the directories of the packages this run
// analyzed, ignoring build constraints, so the answer is the same on every
// platform. It is not a finish step, because a finish step sees only the
// files that were loaded.
func testPackageCoverage(
	root string, setup setup, analyzed []string, blocking []driver.Finding,
) ([]driver.Finding, error) {
	files, err := config.GoFiles(root)
	if err != nil {
		return nil, err
	}
	reported := map[string]bool{}
	for _, finding := range blocking {
		if finding.Category == "TS-T12" {
			reported[filepath.ToSlash(finding.Position.Filename)] = true
		}
	}
	// An external test package reports its own _test path, and a directory
	// can end in _test itself, so both spellings select the directory.
	selected := map[string]bool{}
	for _, pkgPath := range analyzed {
		if rel, inModule := setup.module.Relative(pkgPath); inModule {
			selected[rel] = true
			selected[strings.TrimSuffix(rel, "_test")] = true
		}
	}
	findings := []driver.Finding{}
	for _, dir := range slices.Sorted(maps.Keys(files)) {
		if selected[dir] {
			findings = append(findings, internalTestFiles(root, files[dir], reported)...)
		}
	}
	return findings, nil
}

// internalTestFiles returns a finding for each _test.go file in one
// directory that declares the package under test and is not reported yet.
func internalTestFiles(root string, names []string, reported map[string]bool) []driver.Finding {
	type clause struct {
		name     string
		position token.Position
	}
	clauses := map[string]clause{}
	underTest := map[string]bool{}
	for _, name := range names {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(name)), nil,
			parser.PackageClauseOnly)
		if err != nil {
			// The loader fails the run on a file it loads; one a build
			// constraint excludes fails only when built under its tag.
			continue
		}
		position := fset.Position(file.Name.Pos())
		position.Filename = name
		clauses[name] = clause{name: file.Name.Name, position: position}
		if !strings.HasSuffix(name, "_test.go") {
			underTest[file.Name.Name] = true
		}
	}
	findings := []driver.Finding{}
	for _, name := range slices.Sorted(maps.Keys(clauses)) {
		found := clauses[name]
		if strings.HasSuffix(name, "_test.go") && !reported[name] &&
			testpackage.Internal(found.name, underTest) {
			findings = append(findings, driver.Finding{
				Position: found.position,
				Category: "TS-T12",
				Message:  testpackage.Message(found.name),
			})
		}
	}
	return findings
}
