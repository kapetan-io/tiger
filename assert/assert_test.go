package assert_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thrawn01/tiger/assert"
)

// ID mirrors the inv package pattern: a project-local ~string type passed to
// assert.Invariant, coupling by shape rather than by import.
type ID string

const (
	HeaderSize     ID = "header-size"
	HeaderChecksum ID = "header-checksum"
)

func TestPassingAssertionsAreSilent(t *testing.T) {
	require.NotPanics(t, func() {
		assert.Ok(true, "unused")
		assert.Implies(false, false, "vacuous when antecedent is false")
		assert.Implies(false, true, "vacuous when antecedent is false")
		assert.Implies(true, true, "held")
		assert.Equal("a", "a", "unused")
		assert.Equal(42, 42, "unused")
		assert.Between(5, 5, 9, "low bound is inclusive")
		assert.Between(9, 5, 9, "high bound is inclusive")
		assert.Between(7, 5, 9, "interior")
		assert.Index(0, 4, "first valid index")
		assert.Index(3, 4, "last valid index")
		assert.NoError(nil, "unused")
		assert.Invariant(HeaderSize, true)
	})
}

func TestFailingAssertionMessages(t *testing.T) {
	for _, test := range []struct {
		name string
		fn   func()
		want string
	}{
		{"OkFalse", func() { assert.Ok(false, "boom") }, "assertion failed: boom"},
		{"ImpliesViolated", func() { assert.Implies(true, false, "open implies locked") }, "assertion failed: implication violated: open implies locked"},
		{"EqualDiffers", func() { assert.Equal(2, 3, "entry count") }, "assertion failed: entry count: got 2, want 3"},
		{"BetweenBelowLow", func() { assert.Between(1, 5, 9, "level") }, "assertion failed: level: 1 not in [5, 9]"},
		{"BetweenAboveHigh", func() { assert.Between(10, 5, 9, "level") }, "assertion failed: level: 10 not in [5, 9]"},
		{"IndexNegative", func() { assert.Index(-1, 4, "entry") }, "assertion failed: entry: index -1 out of range [0, 4)"},
		{"IndexAtCount", func() { assert.Index(4, 4, "entry") }, "assertion failed: entry: index 4 out of range [0, 4)"},
		{"NoErrorGotOne", func() { assert.NoError(errors.New("disk gone"), "flush") }, "assertion failed: flush: disk gone"},
		{"Unreachable", func() { assert.Unreachable("unknown opcode") }, "assertion failed: unreachable: unknown opcode"},
		{"FailFormats", func() { assert.Fail("entries %d exceeds max %d", 12, 8) }, "assertion failed: entries 12 exceeds max 8"},
		{"InvariantViolated", func() { assert.Invariant(HeaderSize, false) }, "assertion failed: invariant violated: header-size"},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.PanicsWithValue(t, test.want, test.fn)
		})
	}
}

func TestViolatesDetectsTheNamedViolation(t *testing.T) {
	require.NotPanics(t, func() {
		assert.Violates(HeaderSize, func() {
			assert.Invariant(HeaderSize, false)
		})
	})
}

func TestViolatesFailsWhenNothingIsViolated(t *testing.T) {
	require.PanicsWithValue(t,
		"assertion failed: invariant header-size was not violated",
		func() {
			assert.Violates(HeaderSize, func() {})
		})
}

func TestViolatesReRaisesOtherInvariants(t *testing.T) {
	// A different invariant's failure is not ours to swallow.
	require.PanicsWithValue(t,
		"assertion failed: invariant violated: header-checksum",
		func() {
			assert.Violates(HeaderSize, func() {
				assert.Invariant(HeaderChecksum, false)
			})
		})
}

func TestViolatesReRaisesForeignPanics(t *testing.T) {
	require.PanicsWithValue(t, "unrelated failure", func() {
		assert.Violates(HeaderSize, func() { panic("unrelated failure") })
	})

	// Non-string panic values pass through untouched.
	require.PanicsWithValue(t, 42, func() {
		assert.Violates(HeaderSize, func() { panic(42) })
	})
}
