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

// Analyzer enforces TS-L09: every directive parses, and every escape
// carries a reason and stays visible.
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
				inspect(pass, comment)
				if forbid && isNolint(comment.Text) {
					reportNolint(pass, file, comment)
				}
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

// isNolint reports whether a line comment is a //nolint directive, matched
// the way golangci-lint 2.11 honors it: slashes and spaces after the opening
// slashes are skipped, then nolint must stand alone or be followed by a space
// or colon. Block comments and tabs are not honored there, so they are not
// findings here.
func isNolint(text string) bool {
	body, found := strings.CutPrefix(text, "//")
	if !found {
		return false
	}
	rest, found := strings.CutPrefix(strings.TrimLeft(body, "/ "), "nolint")
	return found && (rest == "" || rest[0] == ' ' || rest[0] == ':')
}

// reportNolint reports a //nolint at the file's package line rather than at
// the comment, so a //nolint on the original line cannot cover its own finding.
func reportNolint(pass *analysis.Pass, file *ast.File, comment *ast.Comment) {
	line := pass.Fset.Position(comment.Pos()).Line
	pass.Report(analysis.Diagnostic{
		Pos:      file.Package,
		Category: "TS-L09",
		Message: "TS-L09: line " + strconv.Itoa(line) + " has a //nolint, and this repository's " +
			"tiger.yaml forbids them (nolint.forbid) — fix the finding it silences instead",
	})
}

// reasonAdvice tells the author what a verb's reason has to say, with an
// example that fits the verb.
func reasonAdvice(verb string) string {
	if verb == "openenum" {
		return "say why the set of values is open, for example //tiger:openenum peers on " +
			"the wire add codes before we upgrade"
	}
	return "write the outside-world constraint that forces this shape, for example //tiger:" +
		verb + " provider offers no bulk endpoint"
}
