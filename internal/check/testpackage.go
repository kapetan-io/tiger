package check

import (
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kapetan-io/tiger/internal/analyzers/testpackage"
	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/module"
)

func testPackageCoverage(
	root string, setup setup, analyzed []string, blocking []driver.Finding,
) ([]driver.Finding, error) {
	files, err := module.GoFiles(root)
	if err != nil {
		return nil, err
	}
	reported := map[string]bool{}
	for _, finding := range blocking {
		if finding.Category == "TS-T12" {
			reported[filepath.ToSlash(finding.Position.Filename)] = true
		}
	}
	selected := map[string]bool{}
	for _, pkgPath := range analyzed {
		if rel, inModule := setup.module.Relative(pkgPath); inModule {
			selected[rel] = true
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
