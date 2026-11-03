// Package directives enforces TS-L09 over the //tiger: namespace.
//
// Two halves at two severities. Blocking: a directive that does not parse —
// an unknown verb (including every //tiger:bounded and //tiger:<rule-id>
// dismissal attempt) or an escape with no reason — because a directive that
// does not parse is an error, never a loophole. With nolint.forbid on in
// tiger.yaml, every //nolint is a blocking finding too, reported at the
// file's package line. Advisory: every well-formed
// escape directive surfaces on every run, so an unverified claim about the
// outside world stays permanently in view instead of sinking into the tree.
package directives

import (
	"errors"
	"go/ast"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/config"
	"github.com/kapetan-io/tiger/internal/directive"
)

var Analyzer = &analysis.Analyzer{
	Name: "directives",
	Doc:  "TS-L09: every tiger directive parses, and every escape carries a reason",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	forbid := config.NolintForbidden()
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				class := directive.Classify(file, comment)
				if class == directive.Tiger {
					inspect(pass, comment)
				}
				if forbid && class == directive.Nolint {
					reportNolint(pass, file, comment)
				}
			}
		}
	}
	return nil, nil
}

func inspect(pass *analysis.Pass, comment *ast.Comment) {
	parsed, err := directive.Parse(comment.Text)
	var unknown *directive.UnknownVerbError
	var unreasoned *directive.MissingReasonError
	var malformed *directive.MalformedArgsError
	switch {
	case errors.As(err, &unknown):
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
			Message: "TS-L09: //tiger:" + parsed.Verb + " has no reason after it — " +
				reasonAdvice(parsed.Verb),
		})
	case errors.As(err, &malformed):
		pass.Report(analysis.Diagnostic{
			Pos:      comment.Pos(),
			Category: "TS-L09",
			Message: "TS-L09: //tiger:" + malformed.Verb + " " + malformed.Detail +
				"; until it parses, this comment enforces nothing",
		})
	case err != nil:
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

func reportNolint(pass *analysis.Pass, file *ast.File, comment *ast.Comment) {
	line := pass.Fset.Position(comment.Pos()).Line
	pass.Report(analysis.Diagnostic{
		Pos:      file.Package,
		Category: "TS-L09",
		Message: "TS-L09: line " + strconv.Itoa(line) + " has a //nolint, and this repository's " +
			"tiger.yaml forbids them (nolint.forbid) — fix the finding it silences instead",
	})
}

func reasonAdvice(verb string) string {
	if verb == "openenum" {
		return "say why the set of values is open, for example //tiger:openenum peers on " +
			"the wire add codes before we upgrade"
	}
	return "write the outside-world constraint that forces this shape, for example //tiger:" +
		verb + " provider offers no bulk endpoint"
}
