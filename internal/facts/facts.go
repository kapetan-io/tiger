// Package facts owns the reported-fact message contract: one format site
// and one extract site for the messages that carry a computed fact's
// directive from an analyzer to a consumer.
//
// The shape is uniform across verbs — "<rule>: <kind> for <function> —
// <directive>" — and the directive substring is byte-exact
// directive.Format output, so extraction ends in directive.Parse and a
// message that does not yield a parseable directive is an error for the
// caller to surface, never a silent skip.
package facts

import (
	"errors"
	"strings"

	"github.com/kapetan-io/tiger/internal/directive"
)

// Fact identifies one reported fact: the rule that computed it, the kind
// of fact, the function it belongs to, and the directive that freezes it.
type Fact struct {
	RuleID    string
	Kind      string
	Function  string
	Directive directive.Directive
}

// Message formats one reported fact.
func Message(f Fact) string {
	return f.RuleID + ": " + f.Kind + " for " + f.Function + " — " + directive.Format(f.Directive)
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
