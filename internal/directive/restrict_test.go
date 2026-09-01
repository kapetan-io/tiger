package directive_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/directive"
)

// TestRestrictRoundTrip exercises the round-trip contract over the restrict
// grammar.
//
// Goal: ParseRestrict(FormatRestrict(r)) returns the identical structure,
// and FormatRestrict prints axes in the canonical order closed-dispatch,
// no-reflect, imports with paths sorted, so a printed declaration is
// byte-identical to the directive that declares it.
func TestRestrictRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name string
		set  directive.Restriction
		text string
	}{
		{
			name: "ClosedDispatchOnly",
			set:  directive.Restriction{ClosedDispatch: true},
			text: "closed-dispatch",
		},
		{
			name: "NoReflectOnly",
			set:  directive.Restriction{NoReflect: true},
			text: "no-reflect",
		},
		{
			name: "ImportsOnly",
			set:  directive.Restriction{Imports: []string{"internal/domain/...", "pkg/store"}},
			text: "imports(internal/domain/..., pkg/store)",
		},
		{
			name: "EveryAxis",
			set: directive.Restriction{
				ClosedDispatch: true,
				NoReflect:      true,
				Imports:        []string{"internal/domain/...", "pkg/store"},
			},
			text: "closed-dispatch, no-reflect, imports(internal/domain/..., pkg/store)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.text, directive.FormatRestrict(test.set))
			parsed, err := directive.ParseRestrict(test.text)
			require.NoError(t, err)
			assert.Equal(t, test.set, parsed)
		})
	}
}

// TestParseRestrictCanonicalizes covers the non-canonical spellings the
// parser accepts.
//
// Goal: axes in any order and unsorted paths parse to the same structure
// the canonical text does.
func TestParseRestrictCanonicalizes(t *testing.T) {
	parsed, err := directive.ParseRestrict(
		"imports(pkg/store, internal/domain/...), no-reflect, closed-dispatch")
	require.NoError(t, err)
	assert.Equal(t, directive.Restriction{
		ClosedDispatch: true,
		NoReflect:      true,
		Imports:        []string{"internal/domain/...", "pkg/store"},
	}, parsed)
	assert.Equal(t,
		"closed-dispatch, no-reflect, imports(internal/domain/..., pkg/store)",
		directive.FormatRestrict(parsed))
}

// TestParseRestrictRejectsMalformedArgs covers the grammar's error cases.
//
// Goal: an unknown axis, unbalanced parentheses, an empty imports(), a
// duplicated axis, a path outside the module, and empty text are all
// errors naming the offending token.
func TestParseRestrictRejectsMalformedArgs(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    string
		wantErr string
	}{
		{"Empty", "", "has nothing after it"},
		{"UnknownAxis", "closed-world", `"closed-world"`},
		{"Unclosed", "imports(internal/domain", "unclosed"},
		{"Unopened", "no-reflect)", "unmatched"},
		{"EmptyImports", "imports()", "imports()"},
		{"BareImports", "imports", "imports"},
		{"DuplicateAxis", "no-reflect, no-reflect", "twice"},
		{"ArgumentsOnBareAxis", "no-reflect(x)", "no-reflect"},
		{"ParentPath", "imports(../other)", `"../other"`},
		{"AbsolutePath", "imports(/internal/domain)", `"/internal/domain"`},
		{"EllipsisInMiddle", "imports(internal/.../domain)", `"internal/.../domain"`},
		{"EmptyItem", "no-reflect,, closed-dispatch", "empty item"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := directive.ParseRestrict(test.args)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

// TestParseValidatesRestrictArgs covers Parse's grammar check on the
// restrict intent verb.
//
// Goal: a malformed //tiger:restrict fails Parse with *MalformedArgsError
// (so directives reports TS-L09), a well-formed one parses, and the other
// intent verbs stay free text.
func TestParseValidatesRestrictArgs(t *testing.T) {
	_, err := directive.Parse("//tiger:restrict closed-world")
	var malformed *directive.MalformedArgsError
	require.ErrorAs(t, err, &malformed)
	assert.Equal(t, "restrict", malformed.Verb)

	parsed, err := directive.Parse("//tiger:restrict no-reflect, imports(internal/domain/...)")
	require.NoError(t, err)
	assert.Equal(t, "no-reflect, imports(internal/domain/...)", parsed.Args)

	_, err = directive.Parse("//tiger:owner anything goes here")
	require.NoError(t, err)
}
