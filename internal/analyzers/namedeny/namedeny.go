// Package namedeny enforces TS-N12 (semantically empty name tokens are
// forbidden) and TS-N13 (no type echo in names).
//
// The spec's own assert package — Ok(cond bool, ...), Unreachable(msg
// string) — is dialect infrastructure whose API the spec itself fixes, so
// the assert package and its external test package are exempt from both
// rules.
package namedeny

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/words"
)

// Analyzer enforces TS-N12 and TS-N13: no semantically empty name tokens, no type echo in names.
var Analyzer = newAnalyzer()

var allowFlag string

func newAnalyzer() *analysis.Analyzer {
	built := &analysis.Analyzer{
		Name: "namedeny",
		Doc:  "TS-N12 and TS-N13: no semantically empty name tokens, no type echo in names.",
		Run:  run,
	}
	built.Flags.StringVar(&allowFlag, "allow", "",
		"comma-separated token=reason entries exempting a TS-N12 token")
	return built
}

// denyTokens is the committed TS-N12 deny dictionary: placeholders that
// survived into the commit and tell the reader nothing about what the
// thing is.
var denyTokens = []string{
	"data", "info", "value", "object", "item", "manager", "handler",
	"helper", "util", "process", "temp", "misc", "common", "base",
	"impl", "wrapper",
}

// denySet holds every deny token plus its literal plural (token + "s"), the
// full set of tokens TS-N12 matches against.
var denySet = buildDenySet()

func buildDenySet() map[string]bool {
	set := make(map[string]bool, len(denyTokens)*2)
	for _, token := range denyTokens {
		set[token] = true
		set[token+"s"] = true
	}
	return set
}

// typeEchoSet is the TS-N13 set of trailing tokens that restate a type
// instead of naming a role.
var typeEchoSet = map[string]bool{
	"str": true, "string": true, "struct": true, "slice": true,
	"list": true, "map": true, "ptr": true, "pointer": true,
	"arr": true, "array": true, "chan": true, "bool": true,
	"int": true, "uint": true, "float": true, "iface": true,
	"interface": true,
}

func run(pass *analysis.Pass) (any, error) {
	name := pass.Pkg.Name()
	if name == "assert" || name == "assert_test" {
		return nil, nil
	}
	allowed, err := parseAllow()
	if err != nil {
		return nil, err
	}
	for _, file := range pass.Files {
		for _, ident := range declaredIdents(file) {
			checkDeny(pass, ident, allowed)
			checkTypeEcho(pass, ident)
		}
	}
	return nil, nil
}

// parseAllow turns the -allow flag into a set of exempted lowercase
// tokens, folded to singular and plural exactly as the deny dictionary is:
// allowing a project-owned term exempts both halves or it exempts neither.
// An entry with no "=reason" half is a configuration error: the per-repo
// allowlist requires a reason, not just a token.
func parseAllow() (map[string]bool, error) {
	allowed := map[string]bool{}
	if allowFlag == "" {
		return allowed, nil
	}
	for _, entry := range strings.Split(allowFlag, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		token, reason, found := strings.Cut(entry, "=")
		if !found || strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf(
				"namedeny: -allow entry %q needs a reason: token=reason", entry)
		}
		lower := strings.ToLower(strings.TrimSpace(token))
		allowed[lower] = true
		allowed[lower+"s"] = true
		allowed[strings.TrimSuffix(lower, "s")] = true
	}
	return allowed, nil
}

// checkDeny reports TS-N12 once for ident's first token that hits the deny
// dictionary and is not on the -allow list.
func checkDeny(pass *analysis.Pass, ident *ast.Ident, allowed map[string]bool) {
	for _, token := range words.Split(ident.Name) {
		lower := strings.ToLower(token)
		if !denySet[lower] || allowed[lower] {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      ident.Pos(),
			Category: "TS-N12",
			Message: fmt.Sprintf(
				"TS-N12: %q uses semantically empty token %q — name what the thing is "+
					"instead of a placeholder (or add -allow=%s=<reason> for a project-owned "+
					"term)", ident.Name, lower, lower),
		})
		return
	}
}

// checkTypeEcho reports TS-N13 when ident has two or more words.Split
// tokens and the last one restates a type.
func checkTypeEcho(pass *analysis.Pass, ident *ast.Ident) {
	tokens := words.Split(ident.Name)
	if len(tokens) < 2 {
		return
	}
	last := tokens[len(tokens)-1]
	if !typeEchoSet[strings.ToLower(last)] {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      ident.Pos(),
		Category: "TS-N13",
		Message: fmt.Sprintf(
			"TS-N13: %q ends with the type-echo token %q — restating the type goes stale "+
				"the moment the type changes; name the role instead (drop the %q suffix or "+
				"replace it with what the value represents)", ident.Name, last, last),
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
