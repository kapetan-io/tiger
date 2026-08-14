package words_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/words"
)

// TestSplit exercises the camelCase, acronym-run, digit, and underscore
// tokenization rules that the naming analyzers depend on.
//
// Goal: an identifier splits into the same words a reader would see,
// including acronym runs handing off to the next Capitalized word and
// digits sticking to the word before them.
func TestSplit(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  []string
	}{
		{name: "single lowercase word", input: "value", want: []string{"value"}},
		{name: "plain camelCase", input: "parseInput", want: []string{"parse", "Input"}},
		{name: "leading acronym run", input: "HTTPServer", want: []string{"HTTP", "Server"}},
		{name: "trailing acronym run", input: "parseURL", want: []string{"parse", "URL"}},
		{name: "acronym run alone", input: "HTTP", want: []string{"HTTP"}},
		{name: "digit sticks to preceding word", input: "userID2", want: []string{"user", "ID2"}},
		{
			name: "digit before capital still splits", input: "item1Value",
			want: []string{"item1", "Value"},
		},
		{name: "underscore boundary", input: "user_id", want: []string{"user", "id"}},
		{
			name: "mixed underscore and camelCase", input: "parse_HTTPServer",
			want: []string{"parse", "HTTP", "Server"},
		},
		{name: "repeated underscores collapse", input: "user__id", want: []string{"user", "id"}},
		{name: "leading and trailing underscore", input: "_user_", want: []string{"user"}},
		{name: "single letter", input: "i", want: []string{"i"}},
		{name: "empty string", input: "", want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, words.Split(test.input))
		})
	}
}
