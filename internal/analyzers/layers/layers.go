// Package layers enforces TS-X03: the package import graph is declared in
// tiger.layers.yaml and every import obeys it.
//
// The driver loads and installs the layer file; this pass only asks it
// where the analyzed package sits and whether each import is listed
// (ADR-0002). Matching is exact, because depguard's prefix match against the
// nearest allow entry cannot express it (ADR-0029). _test.go files are not
// layered, because surface tests cross layers. Entries that match no package,
// and packages with no entry that the run did not load, are checked by tiger
// check (internal/check), which sees the module's directories.
package layers

import (
	"go/ast"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/layerfile"
)

// Analyzer enforces TS-X03: the package import graph is declared and enforced.
var Analyzer = &analysis.Analyzer{
	Name: "layers",
	Doc:  "TS-X03: the package import graph is declared and enforced.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if !layerfile.Installed() {
		return nil, nil
	}
	importer := layerfile.Locate(pass.Pkg.Path())
	if !importer.InModule() || testMain(pass) {
		return nil, nil
	}
	files := sourceFiles(pass)
	if len(files) == 0 {
		return nil, nil
	}
	if !importer.Listed() {
		pass.Report(analysis.Diagnostic{
			Pos:      files[0].Package,
			Category: layerfile.RuleID,
			Message:  layerfile.MissingEntryMessage(importer.Dir()),
		})
		return nil, nil
	}
	for _, file := range files {
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			assert.Ok(err == nil, "an import path that type-checked is a valid string literal")
			verdict := importer.Judge(imported)
			if verdict.Allowed() {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:      spec.Pos(),
				Category: layerfile.RuleID,
				Message:  verdict.Message(),
			})
		}
	}
	return nil, nil
}

// testMain reports whether the package is the main package go test
// synthesizes to run a package's tests, which no layer file can list.
func testMain(pass *analysis.Pass) bool {
	return pass.Pkg.Name() == "main" && strings.HasSuffix(pass.Pkg.Path(), ".test")
}

// sourceFiles returns the package's non-test files.
func sourceFiles(pass *analysis.Pass) []*ast.File {
	files := []*ast.File{}
	for _, file := range pass.Files {
		if !strings.HasSuffix(pass.Fset.Position(file.Package).Filename, "_test.go") {
			files = append(files, file)
		}
	}
	return files
}
