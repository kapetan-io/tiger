// Package namepairs enforces TS-N15: known name pairs use the approved
// half.
//
// The committed table has two match modes, split by a false positive that
// this repository's own dogfood run surfaced (a false positive on a
// blocking rule is a bug in the analyzer, never a directive): src and dst
// are abbreviations wherever they appear, so they match any camelCase
// token — but "in" and "out" are ordinary English prepositions inside
// longer names (endsInUnreachable, ResolvesInRegistry), so they match only
// when the whole identifier is the banned word. The cost is a documented
// known miss: "inBuf"-style abbreviation prefixes are not caught.
package namepairs

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/words"
)

// Analyzer enforces TS-N15: known name pairs use the approved half.
var Analyzer = newAnalyzer()

var pairsFlag string

// tokenPairs is the committed table of banned halves that match any
// camelCase token. Equal length is why the table is built, not what the
// analyzer checks.
var tokenPairs = map[string]string{
	"src": "source",
	"dst": "target",
}

// wholePairs is the committed table of banned halves that match only a
// whole identifier, because the banned word doubles as an English
// preposition inside longer names.
var wholePairs = map[string]string{
	"in":  "input",
	"out": "output",
}

func newAnalyzer() *analysis.Analyzer {
	built := &analysis.Analyzer{
		Name: "namepairs",
		Doc:  "TS-N15: known name pairs use the approved half.",
		Run:  run,
	}
	built.Flags.StringVar(&pairsFlag, "pairs", "",
		"comma-separated bad=good entries extending the committed TS-N15 pair table")
	return built
}

func run(pass *analysis.Pass) (any, error) {
	table, err := pairs()
	if err != nil {
		return nil, err
	}
	for _, file := range pass.Files {
		for _, ident := range declaredIdents(file) {
			check(pass, ident, table)
		}
	}
	return nil, nil
}

// pairs returns the token-matched pair table extended by the -pairs flag.
func pairs() (map[string]string, error) {
	table := make(map[string]string, len(tokenPairs))
	for bad, good := range tokenPairs {
		table[bad] = good
	}
	if pairsFlag == "" {
		return table, nil
	}
	for _, entry := range strings.Split(pairsFlag, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		bad, good, found := strings.Cut(entry, "=")
		if !found || strings.TrimSpace(good) == "" {
			return nil, fmt.Errorf("namepairs: -pairs entry %q needs bad=good", entry)
		}
		table[strings.ToLower(strings.TrimSpace(bad))] = strings.TrimSpace(good)
	}
	return table, nil
}

// check reports TS-N15 once for ident: first against the whole-identifier
// pairs, then for the first camelCase token matching a token pair.
func check(pass *analysis.Pass, ident *ast.Ident, table map[string]string) {
	if good, banned := wholePairs[strings.ToLower(ident.Name)]; banned {
		report(pass, ident, good)
		return
	}
	for _, split := range words.Split(ident.Name) {
		lower := strings.ToLower(split)
		good, banned := table[lower]
		if !banned {
			continue
		}
		report(pass, ident, good)
		return
	}
}

// report emits the TS-N15 finding naming the approved half.
func report(pass *analysis.Pass, ident *ast.Ident, good string) {
	pass.Report(analysis.Diagnostic{
		Pos:      ident.Pos(),
		Category: "TS-N15",
		Message: fmt.Sprintf(
			"TS-N15: %q uses the banned half of a known name pair — related names use the "+
				"approved half so they line up; rename to use %q",
			ident.Name, good),
	})
}

// declaredIdents collects every DECLARED identifier in file: func and
// method names, type names, const/var names (package and local, including
// short := declarations), struct fields, interface methods, parameters,
// and named results. Receivers, "_", import names, and label names are
// never visited by these node types, so they are skipped by construction.
func declaredIdents(file *ast.File) []*ast.Ident {
	var idents []*ast.Ident
	ast.Inspect(file, func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.FuncDecl:
			addIdent(&idents, decl.Name)
		case *ast.FuncType:
			addFieldNames(&idents, decl.Params)
			addFieldNames(&idents, decl.Results)
		case *ast.TypeSpec:
			addIdent(&idents, decl.Name)
		case *ast.StructType:
			addFieldNames(&idents, decl.Fields)
		case *ast.InterfaceType:
			addFieldNames(&idents, decl.Methods)
		case *ast.ValueSpec:
			for _, name := range decl.Names {
				addIdent(&idents, name)
			}
		case *ast.AssignStmt:
			if decl.Tok == token.DEFINE {
				for _, lhs := range decl.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok {
						addIdent(&idents, ident)
					}
				}
			}
		}
		return true
	})
	return idents
}

// addIdent appends ident to idents unless it is the blank identifier.
func addIdent(idents *[]*ast.Ident, ident *ast.Ident) {
	if ident.Name != "_" {
		*idents = append(*idents, ident)
	}
}

// addFieldNames appends every named identifier in list, skipping unnamed
// fields (embedded types, unnamed results) and the blank identifier.
func addFieldNames(idents *[]*ast.Ident, list *ast.FieldList) {
	if list == nil {
		return
	}
	for _, field := range list.List {
		for _, name := range field.Names {
			addIdent(idents, name)
		}
	}
}
