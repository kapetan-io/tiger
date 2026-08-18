package directive_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/directive"
)

// TestRoundTripEveryVerb exercises correctness constraint 2.
//
// Goal: Parse(Format(d)) returns the original directive for every verb in
// the wave-1 vocabulary across representative argument shapes, so report
// output and pin syntax can never drift apart.
func TestRoundTripEveryVerb(t *testing.T) {
	argShapes := []string{
		"",
		"reason",
		"provider offers no bulk endpoint; rate limiter caps us at 10 rps",
		"ENG-147 flaky on windows",
		"closed-dispatch, no-reflect, imports(internal/domain/...)",
		"mutate(r.log, r.checkpoint), io(disk)",
		"len(pending)",
		"internal  double  spaces",
		"unicode: bounded by léngth",
		"punctuation!@#$%^&*()[]{}<>",
	}
	for _, spec := range directive.Verbs() {
		for _, args := range argShapes {
			original := directive.Directive{Verb: spec.Name, Args: args}
			parsed, err := directive.Parse(directive.Format(original))
			if err != nil {
				// Verb-specific validation may reject a shape (an escape
				// without a reason); the round trip must still return the
				// identical structure.
				assert.NotEmpty(t, err.Error())
			}
			assert.Equal(t, original, parsed)
		}
	}
}

// TestParseKnownVerbs covers the accepted forms of the wave-1 vocabulary.
//
// Goal: each verb parses into its verb and canonical trimmed arguments.
func TestParseKnownVerbs(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		want directive.Directive
	}{
		{
			name: "BatchedWithReason",
			text: "//tiger:batched provider offers no bulk endpoint",
			want: directive.Directive{Verb: "batched", Args: "provider offers no bulk endpoint"},
		},
		{
			name: "EffectsPin",
			text: "//tiger:effects mutate(r.log), io(disk)",
			want: directive.Directive{Verb: "effects", Args: "mutate(r.log), io(disk)"},
		},
		{
			name: "HotWithNoArguments",
			text: "//tiger:hot",
			want: directive.Directive{Verb: "hot", Args: ""},
		},
		{
			name: "TrailingWhitespaceIsTrimmed",
			text: "//tiger:variant len(pending)   ",
			want: directive.Directive{Verb: "variant", Args: "len(pending)"},
		},
		{
			name: "TabSeparatesVerbFromArguments",
			text: "//tiger:batched\tno bulk endpoint",
			want: directive.Directive{Verb: "batched", Args: "no bulk endpoint"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := directive.Parse(test.text)
			require.NoError(t, err)
			assert.Equal(t, test.want, parsed)
		})
	}
}

// TestParseRejectsMalformedDirectives covers the closed-vocabulary contract.
//
// Goal: unknown verbs, missing escape reasons, and missing tracker IDs are
// errors — there is no silently meaningless //tiger: comment.
func TestParseRejectsMalformedDirectives(t *testing.T) {
	for _, test := range []struct {
		name    string
		text    string
		wantErr string
	}{
		{
			name:    "UnknownVerb",
			text:    "//tiger:bounded trust me, it converges",
			wantErr: "unknown directive verb //tiger:bounded",
		},
		{
			name:    "RuleIDDeviationAttempt",
			text:    "//tiger:TS-N07 trust me",
			wantErr: "unknown directive verb //tiger:TS-N07",
		},
		{
			name:    "EmptyVerb",
			text:    "//tiger:",
			wantErr: "unknown directive verb //tiger:",
		},
		{
			name:    "EscapeWithoutReason",
			text:    "//tiger:batched",
			wantErr: "//tiger:batched requires a reason",
		},
		{
			name:    "EscapeWithOnlyWhitespace",
			text:    "//tiger:batched   ",
			wantErr: "//tiger:batched requires a reason",
		},
		{
			name:    "WithdrawnSkipVerb",
			text:    "//tiger:skip ENG-42 needs the fake clock from wave 2",
			wantErr: "unknown directive verb //tiger:skip",
		},
		{
			name:    "NotADirective",
			text:    "// tiger:batched prose, not a directive",
			wantErr: "not a tiger directive",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := directive.Parse(test.text)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

// TestParseErrorStillNamesTheVerb covers finding construction.
//
// Goal: a failed parse still returns the offending verb so the directives
// analyzer can name it in the diagnostic.
func TestParseErrorStillNamesTheVerb(t *testing.T) {
	parsed, err := directive.Parse("//tiger:bounded trust me")
	require.Error(t, err)
	assert.Equal(t, "bounded", parsed.Verb)
	assert.Equal(t, "trust me", parsed.Args)
}

// TestIsRecognizesThePrefixExactly covers directive recognition.
//
// Goal: only comments beginning exactly //tiger: are directives; a space
// after the slashes is prose, matching Go's own //go: convention.
func TestIsRecognizesThePrefixExactly(t *testing.T) {
	assert.True(t, directive.Is("//tiger:batched reason"))
	assert.True(t, directive.Is("//tiger:"))
	assert.False(t, directive.Is("// tiger:batched reason"))
	assert.False(t, directive.Is("//tigerstyle:batched reason"))
	assert.False(t, directive.Is("prose mentioning //tiger:batched"))
}

// TestVocabularyKinds covers the declaration-model classification.
//
// Goal: batched is the only escape verb in wave 1, and the pin and intent
// verbs later waves enforce are present for well-formedness validation.
func TestVocabularyKinds(t *testing.T) {
	escapes := 0
	for _, spec := range directive.Verbs() {
		if spec.Kind == directive.KindEscape {
			escapes++
			assert.Equal(t, "batched", spec.Name)
		}
	}
	assert.Equal(t, 1, escapes)

	batched, ok := directive.Lookup("batched")
	require.True(t, ok)
	assert.Equal(t, directive.KindEscape, batched.Kind)

	_, ok = directive.Lookup("bounded")
	assert.False(t, ok)

	// The skip verb was withdrawn when TS-D07 moved from demanding a
	// directive to reporting every skip as a standing advisory.
	_, ok = directive.Lookup("skip")
	assert.False(t, ok)
}

// TestOpenEnumParsesFormatsAndRoundTrips covers the openenum verb TS-S08
// consumes: a same-package intent declaration with no reason argument.
//
// Goal: //tiger:openenum parses to the bare verb, Format reprints it
// canonically, and Parse(Format(d)) returns the identical directive.
func TestOpenEnumParsesFormatsAndRoundTrips(t *testing.T) {
	parsed, err := directive.Parse("//tiger:openenum")
	require.NoError(t, err)
	assert.Equal(t, directive.Directive{Verb: "openenum", Args: ""}, parsed)

	assert.Equal(t, "//tiger:openenum", directive.Format(parsed))

	roundTripped, err := directive.Parse(directive.Format(parsed))
	require.NoError(t, err)
	assert.Equal(t, parsed, roundTripped)
}

// TestOpenEnumIsKindIntentAndRequiresNoReason covers the openenum verb's
// vocabulary entry: it is an intent declaration, not an escape, so it
// parses without a reason argument.
//
// Goal: Lookup reports KindIntent for openenum, and a bare //tiger:openenum
// with no arguments parses without error.
func TestOpenEnumIsKindIntentAndRequiresNoReason(t *testing.T) {
	spec, ok := directive.Lookup("openenum")
	require.True(t, ok)
	assert.Equal(t, directive.KindIntent, spec.Kind)

	_, err := directive.Parse("//tiger:openenum")
	assert.NoError(t, err)
}
