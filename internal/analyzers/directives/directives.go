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
	"strings"

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
	var malformed *directive.MalformedArgsError
	switch {
	case errors.As(err, &unknown):
		// A rule code written as the verb is the unshipped deviation form;
		// naming it generically keeps a second rule code out of the body.
		verb := parsed.Verb
		if strings.HasPrefix(verb, "TS-") {
			verb = "<rule code>"
		}
		pass.Report(analysis.Diagnostic{
			Pos:      comment.Pos(),
			Category: "TS-L09",
			Message: "TS-L09: //tiger:" + verb + " is not a directive tiger recognizes, " +
				"and no directive dismisses a finding — fix the code the finding names, or " +
				"file a bug against tiger",
		})
	case errors.As(err, &unreasoned):
		pass.Report(analysis.Diagnostic{
			Pos:      comment.Pos(),
			Category: "TS-L09",
			Message: "TS-L09: //tiger:" + parsed.Verb + " has no reason after it — write the " +
				"outside-world constraint that forces this shape, for example //tiger:" +
				parsed.Verb + " provider offers no bulk endpoint",
		})
	case errors.As(err, &malformed):
		pass.Report(analysis.Diagnostic{
			Pos:      comment.Pos(),
			Category: "TS-L09",
			Message: "TS-L09: //tiger:" + malformed.Verb + " " + malformed.Detail +
				"; until it parses, this comment enforces nothing",
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
				Message: "TS-L09: " + directive.Format(directive.Directive{
					Verb: parsed.Verb, Args: "",
				}) + " \"" + parsed.Args + "\" waives a rule here; tiger cannot check the " +
					"claim, so this notice stands on every run — remove the directive when " +
					"the constraint no longer holds",
			})
		}
	}
}
