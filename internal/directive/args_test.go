package directive_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/directive"
)

// TestVariantRoundTrip exercises invariant 1 over variant expressions.
//
// Goal: ParseVariant(FormatVariant(v)) returns the identical structure for
// the linear forms the analyzer synthesizes — len terms, differences, and
// integer offsets — preserving term order, which is semantic.
func TestVariantRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
	}{
		{name: "LenOfLocal", text: "len(pending)"},
		{name: "Difference", text: "high - low"},
		{name: "LenMinusIndex", text: "len(queue) - next"},
		{name: "OffsetSum", text: "n - i + 1"},
		{name: "PlainCounter", text: "remaining"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := directive.ParseVariant(test.text)
			require.NoError(t, err)
			assert.Equal(t, test.text, directive.FormatVariant(parsed))
			again, err := directive.ParseVariant(directive.FormatVariant(parsed))
			require.NoError(t, err)
			assert.Equal(t, parsed, again)
		})
	}
}

// TestParseVariantRejectsOutsideThePredicateLanguage covers variant
// validation.
//
// Goal: a variant is a linear integer expression over locals — sums and
// differences of identifiers, len() calls, and integer literals; anything
// else is an error naming the token.
func TestParseVariantRejectsOutsideThePredicateLanguage(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    string
		wantErr string
	}{
		{name: "EmptyArgs", args: "", wantErr: "expression that shrinks on every pass"},
		{name: "CallOutsideLen", args: "size(pending)", wantErr: `"size(pending)"`},
		{name: "TrailingOperator", args: "high -", wantErr: "starts or ends with an operator"},
		{name: "NilAtom", args: "nil", wantErr: `"nil"`},
		{name: "Multiplication", args: "2 * n", wantErr: `"2 * n"`},
		{name: "BrokenPath", args: "len(r..log)", wantErr: "len(r..log), whose path"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := directive.ParseVariant(test.args)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

// TestPredicateRoundTrip exercises invariant 1 over the contracts predicate
// language.
//
// Goal: ParsePredicate(FormatPredicate(p)) returns the identical structure
// across nil-ness, integer ranges, length relations, and invariant IDs.
func TestPredicateRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
	}{
		{name: "NotNil", text: "target != nil"},
		{name: "NilOnTheLeft", text: "nil != target"},
		{name: "LengthEqualsConstant", text: "len(target) == HeaderSizeBytes"},
		{name: "IntegerLowerBound", text: "count >= 0"},
		{name: "NegativeLiteral", text: "offset > -1"},
		{name: "LengthRelation", text: "len(payload) <= len(target)"},
		{name: "FieldPath", text: "result.Checksum != 0"},
		{name: "InvariantID", text: "inv.NoOverdraft"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := directive.ParsePredicate(test.text)
			require.NoError(t, err)
			assert.Equal(t, test.text, directive.FormatPredicate(parsed))
			again, err := directive.ParsePredicate(directive.FormatPredicate(parsed))
			require.NoError(t, err)
			assert.Equal(t, parsed, again)
		})
	}
}

// TestParsePredicateRejectsOutsideThePredicateLanguage covers contracts
// validation.
//
// Goal: the predicate language is closed — nil-ness, integer ranges, length
// relations, invariant IDs — and anything outside it is an error naming the
// token, never a silently meaningless contract.
func TestParsePredicateRejectsOutsideThePredicateLanguage(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    string
		wantErr string
	}{
		{name: "EmptyArgs", args: "", wantErr: "comparison over nil, integers, or lengths"},
		{name: "OrderedNil", args: "count < nil", wantErr: "nil supports only == and !="},
		{
			name: "CallOutsideLen", args: "result.Checksum == checksum(result.Payload)",
			wantErr: `"checksum(result.Payload)"`,
		},
		{name: "BareInteger", args: "42", wantErr: "neither a comparison nor an invariant ID"},
		{name: "DoubleComparison", args: "0 <= i < n", wantErr: `"i < n"`},
		{name: "MissingRightAtom", args: "count >=", wantErr: "missing one side of the comparison"},
		{name: "BothSidesNil", args: "nil == nil", wantErr: "which says nothing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := directive.ParsePredicate(test.args)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

// TestParseValidatesPinVerbArguments covers the grammar's TS-L09 seam.
//
// Goal: Parse applies each pin verb's argument grammar — a malformed pin is
// a *MalformedArgsError carrying the verb and the offending token, while the
// raw verb and argument text still come back for the finding.
func TestParseValidatesPinVerbArguments(t *testing.T) {
	for _, test := range []struct {
		name    string
		text    string
		wantErr string
	}{
		{name: "VariantOutsideTheLanguage", text: "//tiger:variant 2 * n", wantErr: `"2 * n"`},
		{
			name: "RequiresOutsideTheLanguage", text: "//tiger:requires trust me",
			wantErr: `"trust me"`,
		},
		{
			name: "EnsuresOrderedNil", text: "//tiger:ensures result < nil",
			wantErr: "nil supports only == and !=",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := directive.Parse(test.text)
			var malformed *directive.MalformedArgsError
			require.ErrorAs(t, err, &malformed)
			require.ErrorContains(t, err, test.wantErr)
			assert.Equal(t, malformed.Verb, parsed.Verb)
			assert.NotEmpty(t, parsed.Verb)
		})
	}
}

// TestParseAcceptsWellFormedPins covers the accepted pin forms end to end.
//
// Goal: every pin verb's canonical argument text parses through Parse with
// no error, so well-formed pins in real source never fire TS-L09.
func TestParseAcceptsWellFormedPins(t *testing.T) {
	for _, text := range []string{
		"//tiger:variant len(pending)",
		"//tiger:variant high - low",
		"//tiger:requires len(target) == HeaderSizeBytes",
		"//tiger:requires target != nil",
		"//tiger:ensures inv.NoOverdraft",
	} {
		parsed, err := directive.Parse(text)
		require.NoError(t, err, text)
		assert.NotEmpty(t, parsed.Verb)
	}
}
