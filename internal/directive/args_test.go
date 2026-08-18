package directive_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/directive"
)

// TestEffectsRoundTrip exercises invariant 1 over the effect lattice.
//
// Goal: ParseEffects(FormatEffects(set)) returns the identical structure for
// representative sets, and FormatEffects prints the lattice's canonical
// order, so a printed fact is byte-identical to its pin.
func TestEffectsRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name string
		set  directive.EffectSet
		text string
	}{
		{
			name: "Purity",
			set:  directive.EffectSet{},
			text: "none",
		},
		{
			name: "SingleEffect",
			set:  directive.EffectSet{Time: true},
			text: "time",
		},
		{
			name: "EveryLatticeMember",
			set: directive.EffectSet{
				Alloc: true, Block: true, Panic: true, Rand: true,
				Time: true, Spawn: true,
				IO:     []string{"disk", "net"},
				Mutate: []string{"r.checkpoint", "r.log"},
			},
			text: "alloc, io(disk, net), block, panic, rand, time, " +
				"mutate(r.checkpoint, r.log), spawn",
		},
		{
			name: "MutateAndIO",
			set: directive.EffectSet{
				IO:     []string{"disk"},
				Mutate: []string{"r.checkpoint", "r.log"},
			},
			text: "io(disk), mutate(r.checkpoint, r.log)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.text, directive.FormatEffects(test.set))
			parsed, err := directive.ParseEffects(test.text)
			require.NoError(t, err)
			assert.Equal(t, test.set, parsed)
		})
	}
}

// TestParseEffectsNormalizes covers canonicalization.
//
// Goal: hand-written pins in any order, with repeated terms and split io
// lists, parse to the same structure and reformat canonically, so set
// comparison never depends on how the author spelled the pin.
func TestParseEffectsNormalizes(t *testing.T) {
	parsed, err := directive.ParseEffects("mutate(r.log, r.checkpoint), io(disk)")
	require.NoError(t, err)
	assert.Equal(t, directive.EffectSet{
		IO:     []string{"disk"},
		Mutate: []string{"r.checkpoint", "r.log"},
	}, parsed)

	split, err := directive.ParseEffects("io(net), spawn, io(disk), io(net), alloc")
	require.NoError(t, err)
	assert.Equal(t, directive.EffectSet{
		Alloc: true, Spawn: true,
		IO: []string{"disk", "net"},
	}, split)
	assert.Equal(t, "alloc, io(disk, net), spawn", directive.FormatEffects(split))
}

// TestParseEffectsRejectsOutsideTheLattice covers invariant 2.
//
// Goal: any name outside the closed lattice, any io qualifier outside the
// built-in tier, and any structurally broken term is an error naming the
// offending token — a misspelled qualifier is never a silently meaningless
// string.
func TestParseEffectsRejectsOutsideTheLattice(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    string
		wantErr string
	}{
		{name: "UnknownEffect", args: "iio(disk)", wantErr: `unknown effect "iio"`},
		{name: "MisspelledQualifier", args: "io(dsk)", wantErr: `io qualifier "dsk"`},
		{name: "DeclaredTierQualifier", args: "io(database)", wantErr: `io qualifier "database"`},
		{name: "BareIO", args: "io", wantErr: "io requires a qualifier"},
		{name: "BareMutate", args: "mutate", wantErr: "mutate requires a location"},
		{name: "QualifiedTime", args: "time(wall)", wantErr: `"time" takes no arguments`},
		{name: "NoneAmongOthers", args: "none, alloc", wantErr: `"none" states purity`},
		{name: "EmptyArgs", args: "", wantErr: "states a fact"},
		{name: "EmptyTerm", args: "alloc,, spawn", wantErr: "empty effect term"},
		{name: "BadMutatePath", args: "mutate(r..log)", wantErr: `location "r..log"`},
		{name: "UnbalancedParen", args: "io(disk", wantErr: `unclosed "(" in "io(disk"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := directive.ParseEffects(test.args)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

// TestFrameRoundTrip exercises invariant 1 over frame location lists.
//
// Goal: ParseFrame(FormatFrame(locations)) returns the identical sorted
// list, the empty frame is spelled none, and hand-written order normalizes.
func TestFrameRoundTrip(t *testing.T) {
	locations, err := directive.ParseFrame("r.log, r.checkpoint")
	require.NoError(t, err)
	assert.Equal(t, []string{"r.checkpoint", "r.log"}, locations)
	assert.Equal(t, "r.checkpoint, r.log", directive.FormatFrame(locations))

	parsed, err := directive.ParseFrame(directive.FormatFrame(locations))
	require.NoError(t, err)
	assert.Equal(t, locations, parsed)

	empty, err := directive.ParseFrame("none")
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.Equal(t, "none", directive.FormatFrame(nil))
}

// TestParseFrameRejectsMalformedLocations covers frame validation.
//
// Goal: a frame is a comma-separated list of parameter-rooted paths and
// nothing else; broken paths and empty lists are errors naming the token.
func TestParseFrameRejectsMalformedLocations(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    string
		wantErr string
	}{
		{name: "EmptyArgs", args: "", wantErr: "states a fact"},
		{name: "BrokenPath", args: "r..log", wantErr: `location "r..log"`},
		{name: "TrailingDot", args: "r.log.", wantErr: `location "r.log."`},
		{name: "LeadingDigit", args: "0.log", wantErr: `location "0.log"`},
		{name: "NoneAmongOthers", args: "none, r.log", wantErr: `"none" states the empty frame`},
		{name: "EmptyLocation", args: "r.log,,r.checkpoint", wantErr: "empty location"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := directive.ParseFrame(test.args)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

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
		{name: "EmptyArgs", args: "", wantErr: "states a fact"},
		{name: "CallOutsideLen", args: "size(pending)", wantErr: `"size(pending)"`},
		{name: "TrailingOperator", args: "high -", wantErr: "ends in an operator"},
		{name: "NilAtom", args: "nil", wantErr: `"nil"`},
		{name: "Multiplication", args: "2 * n", wantErr: `"2 * n"`},
		{name: "BrokenPath", args: "len(r..log)", wantErr: `"r..log"`},
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
		{name: "EmptyArgs", args: "", wantErr: "states a fact"},
		{name: "OrderedNil", args: "count < nil", wantErr: "nil supports only == and !="},
		{
			name: "CallOutsideLen", args: "result.Checksum == checksum(result.Payload)",
			wantErr: `"checksum(result.Payload)"`,
		},
		{name: "BareInteger", args: "42", wantErr: "compares atoms or names an invariant"},
		{name: "DoubleComparison", args: "0 <= i < n", wantErr: `"i < n"`},
		{name: "MissingRightAtom", args: "count >=", wantErr: "missing an atom"},
		{name: "BothSidesNil", args: "nil == nil", wantErr: "compares nothing"},
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
		{
			name: "MisspelledQualifier", text: "//tiger:effects io(dsk)",
			wantErr: `io qualifier "dsk"`,
		},
		{name: "EmptyEffectsPin", text: "//tiger:effects", wantErr: "states a fact"},
		{name: "BrokenFrame", text: "//tiger:frame r..log", wantErr: `location "r..log"`},
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
		"//tiger:effects mutate(r.log, r.checkpoint), io(disk)",
		"//tiger:effects none",
		"//tiger:frame r.log, r.checkpoint",
		"//tiger:frame none",
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

// TestEffectSetOperations covers the comparison currency analyzers use.
//
// Goal: Equal is exact set equality, Empty spells purity, Union folds
// lattice members and merges qualifier and location lists, and Diff returns
// exactly the members of one set absent from the other — the two directions
// of bidirectional pin enforcement.
func TestEffectSetOperations(t *testing.T) {
	pinned := directive.EffectSet{IO: []string{"disk"}, Time: true}
	computed := directive.EffectSet{IO: []string{"disk", "net"}, Rand: true}

	assert.True(t, directive.EffectSet{}.Empty())
	assert.False(t, pinned.Empty())
	assert.False(t, pinned.Equal(computed))
	assert.True(t, pinned.Equal(directive.EffectSet{Time: true, IO: []string{"disk"}}))

	undeclared := computed.Diff(pinned)
	assert.Equal(t, "io(net), rand", directive.FormatEffects(undeclared))
	absent := pinned.Diff(computed)
	assert.Equal(t, "time", directive.FormatEffects(absent))

	union := pinned.Union(computed)
	assert.Equal(t, "io(disk, net), rand, time", directive.FormatEffects(union))
}
