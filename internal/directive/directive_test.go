package directive_test

import (
	"go/parser"
	"go/token"
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
			name: "RequiresPin",
			text: "//tiger:requires len(target) == HeaderSizeBytes",
			want: directive.Directive{Verb: "requires", Args: "len(target) == HeaderSizeBytes"},
		},
		{
			name: "OpenEnumWithReason",
			text: "//tiger:openenum peers add codes before we upgrade",
			want: directive.Directive{Verb: "openenum", Args: "peers add codes before we upgrade"},
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

// TestClassify pins the class every reader of a comment binds to.
//
// Goal: the same text means the same thing to the comment ban, the pin
// readers and the nolint check; only the Header class depends on position.
func TestClassify(t *testing.T) {
	const pkg = "package p\n"
	for _, test := range []struct {
		name string
		code string
		want directive.Class
	}{
		{name: "TigerDirective", code: pkg + "//tiger:batched reason\n", want: directive.Tiger},
		{name: "BareTigerPrefix", code: pkg + "//tiger:\n", want: directive.Tiger},
		{
			name: "SpaceAfterSlashesIsProse",
			code: pkg + "// tiger:batched reason\n",
			want: directive.Prose,
		},
		{name: "LongerWordIsProse", code: pkg + "//tigerstyle:batched x\n", want: directive.Prose},
		{name: "GoGenerate", code: pkg + "//go:generate x\n", want: directive.GoDirective},
		{name: "GoEmbed", code: pkg + "//go:embed f\n", want: directive.GoDirective},
		{name: "Line", code: pkg + "//line a.go:10\n", want: directive.GoDirective},
		{name: "BlockLine", code: pkg + "/*line a.go:10*/\n", want: directive.GoDirective},
		{name: "Export", code: pkg + "//export F\n", want: directive.GoDirective},
		{name: "Extern", code: pkg + "//extern f\n", want: directive.GoDirective},
		{name: "OtherWordColonWord", code: pkg + "//easyjson:json\n", want: directive.Prose},
		{name: "WhyDirective", code: pkg + "//why:x\n", want: directive.Prose},
		{name: "LintIgnore", code: pkg + "//lint:ignore SA1 x\n", want: directive.Prose},
		{name: "BareNolint", code: pkg + "//nolint\n", want: directive.Nolint},
		{
			name: "NolintWithLinter",
			code: pkg + "//nolint:errcheck // reason\n",
			want: directive.Nolint,
		},
		{name: "SpacedNolint", code: pkg + "// nolint\n", want: directive.Nolint},
		{name: "TripleSlashNolint", code: pkg + "///nolint\n", want: directive.Nolint},
		{name: "NolintPrefixOfAWord", code: pkg + "//nolinting\n", want: directive.Prose},
		{name: "PlainProse", code: pkg + "// x\n", want: directive.Prose},
		{name: "BlockProse", code: pkg + "/* x */\n", want: directive.Prose},
		{name: "BareSlashes", code: pkg + "//\n", want: directive.Prose},
		{name: "TrailingOnPackageLine", code: "package p // x\n", want: directive.Prose},
		{name: "AbovePackage", code: "// x\npackage p\n", want: directive.Header},
		{name: "BuildTag", code: "//go:build linux\n\npackage p\n", want: directive.GoDirective},
		{
			name: "TigerAbovePackage",
			code: "//tiger:batched r\n\npackage p\n",
			want: directive.Tiger,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "p.go", test.code,
				parser.ParseComments)
			require.NoError(t, err)
			require.Len(t, file.Comments, 1)
			assert.Equal(t, test.want, directive.Classify(file, file.Comments[0].List[0]))
		})
	}
}

// TestVocabularyKinds covers the declaration-model classification.
//
// Goal: batched and openenum are the escape verbs, and the pin verbs are
// present for well-formedness validation.
func TestVocabularyKinds(t *testing.T) {
	escapes := []string{}
	for _, spec := range directive.Verbs() {
		if spec.Kind == directive.KindEscape {
			escapes = append(escapes, spec.Name)
		}
	}
	assert.Equal(t, []string{"batched", "openenum"}, escapes)

	batched, ok := directive.Lookup("batched")
	require.True(t, ok)
	assert.Equal(t, directive.KindEscape, batched.Kind)

	_, ok = directive.Lookup("bounded")
	assert.False(t, ok)

	// ADR-0026 retired the effect and frame pins in favor of tiger changed.
	for _, retired := range []string{"effects", "frame"} {
		_, ok = directive.Lookup(retired)
		assert.False(t, ok)
	}

	// The skip verb was withdrawn when TS-D07 moved from demanding a
	// directive to reporting every skip as a standing advisory.
	_, ok = directive.Lookup("skip")
	assert.False(t, ok)
}

// TestOpenEnumParsesFormatsAndRoundTrips covers the openenum verb TS-S08
// consumes: a same-package escape declaration that carries a reason.
//
// Goal: //tiger:openenum <reason> parses to the verb and reason, Format
// reprints it canonically, and Parse(Format(d)) returns the identical directive.
func TestOpenEnumParsesFormatsAndRoundTrips(t *testing.T) {
	parsed, err := directive.Parse("//tiger:openenum wire peers add codes before we upgrade")
	require.NoError(t, err)
	assert.Equal(t, directive.Directive{
		Verb: "openenum", Args: "wire peers add codes before we upgrade",
	}, parsed)

	assert.Equal(t, "//tiger:openenum wire peers add codes before we upgrade",
		directive.Format(parsed))

	roundTripped, err := directive.Parse(directive.Format(parsed))
	require.NoError(t, err)
	assert.Equal(t, parsed, roundTripped)
}

// TestOpenEnumIsKindEscapeAndRequiresAReason covers the openenum verb's
// vocabulary entry: tiger cannot check that an enum is open, so it is an
// escape and needs a reason.
//
// Goal: Lookup reports KindEscape for openenum, and a bare //tiger:openenum
// fails with a missing-reason error.
func TestOpenEnumIsKindEscapeAndRequiresAReason(t *testing.T) {
	spec, ok := directive.Lookup("openenum")
	require.True(t, ok)
	assert.Equal(t, directive.KindEscape, spec.Kind)

	_, err := directive.Parse("//tiger:openenum")
	var missing *directive.MissingReasonError
	assert.ErrorAs(t, err, &missing)
}
