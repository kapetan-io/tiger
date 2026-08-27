// Package facts owns the reported-fact message contract: one format site
// and one extract site for the messages that carry a computed fact's
// directive from an analyzer to a consumer.
//
// A fact message says what tiger found in plain words, that there is
// nothing to fix, and ends with the directive that would freeze the fact.
// The directive substring is byte-exact directive.Format output and is
// always the tail of the line, so extraction ends in directive.Parse and a
// message that does not yield a parseable directive is an error for the
// caller to surface, never a silent skip.
package facts

import (
	"errors"
	"strings"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/directive"
)

// Fact identifies one reported fact: the rule that computed it, the
// function it belongs to, and the directive that freezes it. The
// directive's verb selects the wording.
type Fact struct {
	RuleID    string
	Function  string
	Directive directive.Directive
}

// Message formats one reported fact for the --show-facts channel.
func Message(f Fact) string {
	pin := directive.Format(f.Directive)
	tail := " — nothing to fix; to make tiger fail the build if that changes, add " + pin
	switch f.Directive.Verb {
	case "effects":
		if f.Directive.Args == "none" {
			return f.RuleID + ": " + f.Function + " has no effects" + tail
		}
		return f.RuleID + ": " + f.Function + "'s effects are " + f.Directive.Args +
			" — nothing to fix; to make tiger fail the build if they change, add " + pin
	case "frame":
		writes := "writes " + f.Directive.Args
		if f.Directive.Args == "none" {
			writes = "writes nothing"
		}
		return f.RuleID + ": " + f.Function + " " + writes +
			" through its receiver or parameters" + tail
	case "variant":
		return f.RuleID + ": this loop in " + f.Function + " ends because " +
			f.Directive.Args + " shrinks on every pass" + tail
	default:
		assert.Unreachable("facts.Message: verb " + f.Directive.Verb + " carries no fact")
		return ""
	}
}

// Extract returns the directive embedded in a reported fact's message.
func Extract(message string) (directive.Directive, error) {
	at := strings.Index(message, directive.Prefix)
	if at < 0 {
		return directive.Directive{Verb: "", Args: ""},
			errors.New("no directive in fact message: " + message)
	}
	return directive.Parse(message[at:])
}
