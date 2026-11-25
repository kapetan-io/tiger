// Package facts owns the reported-fact message contract: one format site
// and one extract site for the messages that carry a computed fact's
// directive from an analyzer to a consumer.
package facts

import (
	"errors"
	"strings"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/directive"
)

type Fact struct {
	RuleID    string
	Function  string
	Directive directive.Directive
}

func Message(f Fact) string {
	pin := directive.Format(f.Directive)
	tail := " — nothing to fix; to make tiger fail the build if that changes, add " + pin
	switch f.Directive.Verb {
	case "variant":
		return f.RuleID + ": this loop in " + f.Function + " ends because " +
			f.Directive.Args + " shrinks on every pass" + tail
	default:
		assert.Unreachable("facts.Message: verb " + f.Directive.Verb + " carries no fact")
		return ""
	}
}

func Extract(message string) (directive.Directive, error) {
	at := strings.Index(message, directive.Prefix)
	if at < 0 {
		return directive.Directive{Verb: "", Args: ""},
			errors.New("no directive in fact message: " + message)
	}
	return directive.Parse(message[at:])
}
