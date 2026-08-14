// The TS-T10 failure mode: a table-driven test case struct with no name
// field, so a failing case cannot say what it was.
package fixture

import "testing"

func TestSumUnnamed(t *testing.T) {
	cases := []struct { // want `TS-T10: table-driven test case struct has no name field`
		input int
		want  int
	}{
		{input: 1, want: 1},
		{input: 2, want: 2},
	}
	for _, test := range cases {
		if test.input != test.want {
			t.Fatalf("got %d, want %d", test.input, test.want)
		}
	}
}

func BenchmarkAddUnnamed(b *testing.B) {
	cases := []struct { // want `TS-T10: table-driven test case struct has no name field`
		input int
	}{
		{input: 1},
		{input: 2},
	}
	for _, test := range cases {
		for i := 0; i < b.N; i++ {
			_ = test.input + 1
		}
	}
}
