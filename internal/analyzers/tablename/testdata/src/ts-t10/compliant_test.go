// The compliant rewrites: a "name" or "Name" field explains each case, and a
// map[string]struct table is named by construction.
package fixture

import "testing"

func TestSumNamed(t *testing.T) {
	cases := []struct {
		name  string
		input int
		want  int
	}{
		{name: "one", input: 1, want: 1},
		{name: "two", input: 2, want: 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.input != test.want {
				t.Fatalf("got %d, want %d", test.input, test.want)
			}
		})
	}
}

func TestSumCapitalName(t *testing.T) {
	cases := []struct {
		Name  string
		Input int
	}{
		{Name: "one", Input: 1},
		{Name: "two", Input: 2},
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			_ = test.Input
		})
	}
}

func TestSumMap(t *testing.T) {
	cases := map[string]struct {
		input int
		want  int
	}{
		"one": {input: 1, want: 1},
		"two": {input: 2, want: 2},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if test.input != test.want {
				t.Fatalf("got %d, want %d", test.input, test.want)
			}
		})
	}
}
