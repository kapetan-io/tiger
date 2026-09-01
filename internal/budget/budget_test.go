package budget_test

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/budget"
)

// codes is the advisory code set the tests load against, sorted.
var codes = []string{"TS-D07", "TS-L09"}

// allowed is codes as the set Load validates against.
var allowed = map[string]bool{"TS-D07": true, "TS-L09": true}

// fixture is one budget file's text in the directory it is written to.
type fixture struct {
	dir  string
	rows string
}

// load writes the fixture and loads it back.
func (f fixture) load(t *testing.T) *budget.File {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, budget.FileName), []byte(f.rows), 0o644))
	loaded, err := budget.Load(f.dir, allowed)
	require.NoError(t, err)
	return loaded
}

// TestLowerNeverRaisesARow covers invariant I2 with random inputs: this is
// the one test the blueprint admits below the CLI surface, because the
// writer's arithmetic is the load-bearing guarantee of ADR-0011.
//
// Goal: for random existing budgets and random counts over random package
// sets, every row Lower produces equals its count for a new row and is at
// most the row it replaced for an existing one, is never 0, and rows for
// packages outside the run are untouched.
func TestLowerNeverRaisesARow(t *testing.T) {
	random := rand.New(rand.NewPCG(153, 153))
	packages := []string{".", "internal/a", "internal/b", "internal/c", "internal/d"}
	const rounds = 500
	dir := t.TempDir()
	for round := 0; round < rounds; round++ {
		rows := ""
		existing := map[budget.Key]int{}
		for _, pkg := range packages {
			if random.IntN(3) == 0 {
				continue
			}
			block := ""
			for _, code := range codes {
				if random.IntN(2) == 0 {
					continue
				}
				number := random.IntN(6)
				existing[budget.Key{Package: pkg, Code: code}] = number
				block += fmt.Sprintf("  %s: %d\n", code, number)
			}
			if block != "" {
				rows += pkg + ":\n" + block
			}
		}
		loaded := fixture{dir: dir, rows: rows}.load(t)
		counts := budget.Counts{}
		analyzed := []string{}
		inRun := map[string]bool{}
		for _, pkg := range packages {
			if random.IntN(2) == 0 {
				continue
			}
			analyzed = append(analyzed, pkg)
			inRun[pkg] = true
			for _, code := range codes {
				counts[budget.Key{Package: pkg, Code: code}] = random.IntN(6)
			}
		}
		lowered := loaded.Lower(counts, budget.Run{Packages: analyzed, Codes: codes}).Rows()
		for _, pkg := range packages {
			for _, code := range codes {
				key := budget.Key{Package: pkg, Code: code}
				before, had := existing[key]
				want := before
				if inRun[pkg] {
					want = counts[key]
					if had {
						want = min(before, want)
					}
				}
				got, found := lowered[key]
				if want == 0 {
					assert.False(t, found, key)
					continue
				}
				assert.Equal(t, want, got, key)
			}
		}
	}
}

// TestEncodeIsDeterministic covers invariant I4.
//
// Goal: the same rows encode to the same bytes, sorted, two-space
// indented, with a trailing newline, whatever order they were built in.
func TestEncodeIsDeterministic(t *testing.T) {
	first := fixture{
		dir:  t.TempDir(),
		rows: "internal/b:\n  TS-L09: 2\n  TS-D07: 1\ninternal/a:\n  TS-D07: 3\n",
	}.load(t)
	second := fixture{
		dir:  t.TempDir(),
		rows: "internal/a:\n  TS-D07: 3\ninternal/b:\n  TS-D07: 1\n  TS-L09: 2\n",
	}.load(t)
	want := "internal/a:\n  TS-D07: 3\ninternal/b:\n  TS-D07: 1\n  TS-L09: 2\n"
	assert.Equal(t, want, string(first.Encode()))
	assert.Equal(t, want, string(second.Encode()))
}

// TestWriteLeavesNoTempFile covers behavioral constraint B2's visible half.
//
// Goal: after Write the directory holds the budget file and nothing else,
// and the file's bytes are the encoding.
func TestWriteLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	loaded := fixture{dir: dir, rows: "internal/a:\n  TS-D07: 3\n"}.load(t)
	require.NoError(t, loaded.Write(dir))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, budget.FileName, entries[0].Name())
	written, err := os.ReadFile(filepath.Join(dir, budget.FileName))
	require.NoError(t, err)
	assert.Equal(t, loaded.Encode(), written)
}
