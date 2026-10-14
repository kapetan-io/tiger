package pattern_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/pattern"
)

const goodCard = `# sample

Summary line.

## Rules

TS-C02, TS-S03

## Gap

What the rules miss.

## Bug

The bug behind it.

## Checklist

- Does it do the thing?
- Is it tested?
`

// TestLoadReadsCardAndFiles loads one well-formed pattern.
//
// Goal: the card sections, the rule codes and the Go files, tests flagged and
// sorted by path, come back as written.
func TestLoadReadsCardAndFiles(t *testing.T) {
	all, err := pattern.Load(fstest.MapFS{
		"sample/PATTERN.md":     {Data: []byte(goodCard)},
		"sample/b_test.go":      {Data: []byte("package sample_test\n")},
		"sample/a.go":           {Data: []byte("package sample\n")},
		"sample/inner/inner.go": {Data: []byte("package inner\n")},
		"stray.txt":             {Data: []byte("not a pattern")},
	})
	require.NoError(t, err)
	require.Len(t, all, 1)
	got := all[0]
	assert.Equal(t, "sample", got.Name)
	assert.Equal(t, "Summary line.", got.Summary)
	assert.Equal(t, []string{"TS-C02", "TS-S03"}, got.Rules)
	assert.Equal(t, "What the rules miss.", got.Gap)
	assert.Equal(t, "The bug behind it.", got.Bug)
	assert.Equal(t, []string{"Does it do the thing?", "Is it tested?"}, got.Checklist)
	require.Len(t, got.Files, 3)
	assert.Equal(t, "a.go", got.Files[0].Path)
	assert.False(t, got.Files[0].Test)
	assert.Equal(t, "b_test.go", got.Files[1].Path)
	assert.True(t, got.Files[1].Test)
	assert.Equal(t, "inner/inner.go", got.Files[2].Path)
}

// bodies holds the text under each card section.
type bodies struct {
	Rules     string
	Gap       string
	Bug       string
	Checklist string
}

// cardWith builds a card with the given section bodies.
func cardWith(text bodies) string {
	return "# sample\n\nSummary.\n\n## Rules\n\n" + text.Rules + "\n\n## Gap\n\n" + text.Gap +
		"\n\n## Bug\n\n" + text.Bug + "\n\n## Checklist\n\n" + text.Checklist + "\n"
}

// TestLoadJoinsAWrappedChecklistItem wraps one checklist question over two lines.
//
// Goal: the continuation line joins its item, so a long question still reads as one.
func TestLoadJoinsAWrappedChecklistItem(t *testing.T) {
	card := cardWith(bodies{
		Rules: "TS-C02", Gap: "g", Bug: "b",
		Checklist: "- Does the long question\n  wrap onto a second line?\n- Is it short?",
	})
	all, err := pattern.Load(fstest.MapFS{
		"sample/PATTERN.md": {Data: []byte(card)},
		"sample/a.go":       {Data: []byte("package sample\n")},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Does the long question wrap onto a second line?", "Is it short?"},
		all[0].Checklist)
}

// TestLoadRejectsAPatternItCannotPrint covers the malformed cards.
//
// Goal: a missing card, a missing section, a bad rule code, a checklist item
// that is not a question and a directory with no Go files each fail the load
// naming the pattern.
func TestLoadRejectsAPatternItCannotPrint(t *testing.T) {
	good := fstest.MapFile{Data: []byte("package sample\n")}
	for _, test := range []struct {
		name string
		card string
		fsys fstest.MapFS
		want string
	}{
		{
			name: "NoCard",
			fsys: fstest.MapFS{"sample/a.go": &good},
			want: "pattern sample: open sample/PATTERN.md",
		},
		{
			name: "NoSummary",
			card: "# sample\n\n## Rules\n\nTS-C02\n",
			want: "no summary paragraph",
		},
		{
			name: "EmptyGap",
			card: cardWith(bodies{Rules: "TS-C02", Bug: "b", Checklist: "- Q?"}),
			want: `section "Gap" is missing or empty`,
		},
		{
			name: "RuleProse",
			card: cardWith(
				bodies{Rules: "TS-C02 and TS-S03", Gap: "g", Bug: "b", Checklist: "- Q?"},
			),
			want: `"and" in the Rules section is not a rule code`,
		},
		{
			name: "StatementNotQuestion",
			card: cardWith(
				bodies{Rules: "TS-C02", Gap: "g", Bug: "b", Checklist: "- Always do it."},
			),
			want: `checklist item "Always do it." is not a question`,
		},
		{
			name: "DuplicateRule",
			card: cardWith(bodies{Rules: "TS-C02, TS-C02", Gap: "g", Bug: "b", Checklist: "- Q?"}),
			want: `"TS-C02" is listed twice`,
		},
		{
			name: "NoGoFiles",
			card: goodCard,
			fsys: fstest.MapFS{},
			want: "pattern sample: no Go files",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fsys := test.fsys
			if fsys == nil {
				fsys = fstest.MapFS{"sample/a.go": &good}
			}
			if test.card != "" || test.name == "NoGoFiles" {
				fsys["sample/PATTERN.md"] = &fstest.MapFile{Data: []byte(test.card)}
			}
			_, err := pattern.Load(fsys)
			require.ErrorContains(t, err, test.want)
		})
	}
}

// TestFindAndBehindLookUpPatterns covers the two lookups the CLI uses.
//
// Goal: Find returns a pattern by name and Behind returns every pattern
// backing a rule code, in load order.
func TestFindAndBehindLookUpPatterns(t *testing.T) {
	all := []pattern.Pattern{
		{Name: "a", Rules: []string{"TS-C02"}},
		{Name: "b", Rules: []string{"TS-C02", "TS-S03"}},
	}
	found, ok := pattern.Find(all, "b")
	require.True(t, ok)
	assert.Equal(t, "b", found.Name)
	_, ok = pattern.Find(all, "z")
	assert.False(t, ok)
	assert.Len(t, pattern.Behind(all, "TS-C02"), 2)
	assert.Len(t, pattern.Behind(all, "TS-S03"), 1)
	assert.Empty(t, pattern.Behind(all, "TS-Z99"))
}
