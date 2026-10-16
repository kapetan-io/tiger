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
// embedded, for the variant pin.
func TestMessageAndExtractRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name     string
		ruleID   string
		function string
		d        directive.Directive
		want     string
	}{
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
			message: "TS-V01: drain ends — nothing to fix",
			wantErr: "no directive in fact message",
		},
		{
			name:    "UnknownVerb",
			message: "TS-V01: drain ends — nothing to fix; add //tiger:bogus none",
			wantErr: "unknown directive verb",
		},
		{
			name:    "MalformedArgs",
			message: "TS-V01: drain ends — add //tiger:variant 2 * n",
			wantErr: "//tiger:variant",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := facts.Extract(test.message)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}
