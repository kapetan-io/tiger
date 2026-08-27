package facts_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/directive"
	"github.com/kapetan-io/tiger/internal/facts"
)

// TestMessageAndExtractRoundTrip covers the fact-message contract in both
// directions.
//
// Goal: every formatted message yields back the exact directive it
// embedded, for all three pinnable verbs.
func TestMessageAndExtractRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name     string
		ruleID   string
		function string
		d        directive.Directive
		want     string
	}{
		{
			name:     "Effects",
			ruleID:   "TS-F01",
			function: "Append",
			d:        directive.Directive{Verb: "effects", Args: "alloc, mutate(r.log)"},
			want: "TS-F01: Append's effects are alloc, mutate(r.log) — nothing to fix; to make " +
				"tiger fail the build if they change, add //tiger:effects alloc, mutate(r.log)",
		},
		{
			name:     "Frame",
			ruleID:   "TS-F07",
			function: "Append",
			d:        directive.Directive{Verb: "frame", Args: "r.log"},
			want: "TS-F07: Append writes r.log through its receiver or parameters — nothing to " +
				"fix; to make tiger fail the build if that changes, add //tiger:frame r.log",
		},
		{
			name:     "Variant",
			ruleID:   "TS-V01",
			function: "drain",
			d:        directive.Directive{Verb: "variant", Args: "len(pending)"},
			want: "TS-V01: this loop in drain ends because len(pending) shrinks on every pass — " +
				"nothing to fix; to make tiger fail the build if that changes, add " +
				"//tiger:variant len(pending)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := facts.Message(facts.Fact{
				RuleID: test.ruleID, Function: test.function, Directive: test.d,
			})
			assert.Equal(t, test.want, message)

			extracted, err := facts.Extract(message)
			require.NoError(t, err)
			assert.Equal(t, test.d, extracted)
		})
	}
}

// TestExtractErrors covers the non-silent failure contract.
//
// Goal: a message with no directive, or one whose directive fails the
// grammar, is an error — never an empty success.
func TestExtractErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		message string
		wantErr string
	}{
		{
			name:    "NoDirective",
			message: "TS-F01: Append has no effects — nothing to fix",
			wantErr: "no directive in fact message",
		},
		{
			name:    "UnknownVerb",
			message: "TS-F01: Append has no effects — nothing to fix; add //tiger:bogus none",
			wantErr: "unknown directive verb",
		},
		{
			name:    "MalformedArgs",
			message: "TS-F01: Append's effects are notaneffect — add //tiger:effects notaneffect",
			wantErr: "//tiger:effects",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := facts.Extract(test.message)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}
