// Package directives enforces TS-L09 over the //tiger: namespace.
//
// Two halves at two severities. Blocking: a directive that does not parse —
// an unknown verb (including every //tiger:bounded and //tiger:<rule-id>
// dismissal attempt) or an escape with no reason — because a directive that
// does not parse is an error, never a loophole. Advisory: every well-formed
// escape directive surfaces on every run, so an unverified claim about the
// outside world stays permanently in view instead of sinking into the tree.
package directives

import (
	"errors"
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/directive"
)

// Analyzer enforces TS-L09: every directive parses, and every escape
// carries a reason and stays visible.
var Analyzer = &analysis.Analyzer{
	Name: "directives",
	Doc:  "TS-L09: every tiger directive parses, and every escape carries a reason",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				inspect(pass, comment)
			}
		}
	}
	return nil, nil
}

// inspect reports one comment's directive findings, if it is a directive.
func inspect(pass *analysis.Pass, comment *ast.Comment) {
	if !directive.Is(comment.Text) {
		return
	}
	parsed, err := directive.Parse(comment.Text)
	var unknown *directive.UnknownVerbError
	var unreasoned *directive.MissingReasonError
	switch {
	case errors.As(err, &unknown):
		pass.Report(analysis.Diagnostic{
			Pos:      comment.Pos(),
			Category: "TS-L09",
			Message: "TS-L09: unknown directive verb //tiger:" + parsed.Verb +
				" — the verb vocabulary is closed and there is no dismissal directive; " +
				"fix the code the finding names, or file a bug against the analyzer",
		})
	case errors.As(err, &unreasoned):
		pass.Report(analysis.Diagnostic{
			Pos:      comment.Pos(),
			Category: "TS-L09",
			Message: "TS-L09: //tiger:" + parsed.Verb + " carries no reason — an escape " +
				"without a reason is a rule silently deleted; state the constraint of the " +
				"outside world that forces this shape",
		})
	case err != nil:
		// Is() accepted the prefix, so Parse can only fail one of the ways
		// above; a new failure mode must land in this switch.
		return
	default:
		spec, _ := directive.Lookup(parsed.Verb)
		if spec.Kind == directive.KindEscape {
			pass.Report(analysis.Diagnostic{
				Pos:      comment.Pos(),
				Category: "TS-L09-escape",
				Message: "TS-L09: escape " + directive.Format(directive.Directive{
					Verb: parsed.Verb, Args: "",
				}) + " — \"" + parsed.Args + "\" (unverified claim; standing review)",
			})
		}
	}
}
