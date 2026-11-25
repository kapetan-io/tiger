// Package layers enforces TS-X03: the package import graph is declared in
// tiger.layers.yaml and every import obeys it.
package layers

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/layerfile"
)

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
		reportUses(pass, importer, file)
	}
	return nil, nil
}

func reportUses(pass *analysis.Pass, importer layerfile.Importer, file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		ident, isIdent := node.(*ast.Ident)
		if !isIdent {
			return true
		}
		used, isFunc := pass.TypesInfo.Uses[ident].(*types.Func)
		if !isFunc || used.Pkg() == nil {
			return true
		}
		if verdict := importer.JudgeUse(used.FullName()); !verdict.Allowed() {
			pass.Report(analysis.Diagnostic{
				Pos:      ident.Pos(),
				Category: layerfile.RuleID,
				Message:  verdict.Message(),
			})
		}
		return true
	})
}

func testMain(pass *analysis.Pass) bool {
	return pass.Pkg.Name() == "main" && strings.HasSuffix(pass.Pkg.Path(), ".test")
}

func sourceFiles(pass *analysis.Pass) []*ast.File {
	files := []*ast.File{}
	for _, file := range pass.Files {
		if !strings.HasSuffix(pass.Fset.Position(file.Package).Filename, "_test.go") {
			files = append(files, file)
		}
	}
	return files
}
