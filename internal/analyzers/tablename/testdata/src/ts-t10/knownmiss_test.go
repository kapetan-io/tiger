// known-miss: a slice of a declared (named) struct type is not examined, so
// a named struct type with no name field silently escapes TS-T10 even
// though it has the same unreadable-failure-output problem as the anonymous
// case.
package fixture

import "testing"

type sumCase struct {
	input int
	want  int
}

func TestSumNamedType(t *testing.T) {
	cases := []sumCase{
		{input: 1, want: 1},
		{input: 2, want: 2},
	}
	for _, test := range cases {
		if test.input != test.want {
			t.Fatalf("got %d, want %d", test.input, test.want)
		}
	}
}
