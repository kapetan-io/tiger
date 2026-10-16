package cli_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// commentLimit repeats the renderer's cap so a change to it fails here.
const commentLimit = 60000

// TestChangedMarkdownGoldens covers the exact bytes of the pull request comment.
//
// Goal: the markdown report of every case equals its golden file, exits 0,
// and writes nothing to stderr.
func TestChangedMarkdownGoldens(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := fixture(t, name).changed("--format", "markdown")
			assert.Equal(t, cli.ExitClean, got.code)
			assert.Empty(t, got.stderr)
			golden(t, name, "markdown.golden", got)
		})
	}
}

// TestChangedMarkdownStartsWithTheMarker covers how CI finds its one comment.
//
// Goal: line 1 is the marker, the title names the base ref and both hashes,
// and a dirty tree is labeled in the title.
func TestChangedMarkdownStartsWithTheMarker(t *testing.T) {
	c := fixture(t, "call-added")
	got := c.changed("--format", "markdown")
	require.Equal(t, cli.ExitClean, got.code)
	lines := strings.Split(got.stdout, "\n")
	assert.Equal(t, "<!-- tiger-changed -->", lines[0])
	assert.Equal(t, fmt.Sprintf("#### tiger changed: `main` (%s) → `%s`",
		c.short("main"), c.short("HEAD")), lines[1])

	c.write("README.md", "edited\n")
	dirty := c.changed("--format", "markdown")
	assert.Contains(t, dirty.stdout, "→ `"+c.short("HEAD")+"` + uncommitted changes\n")
}

// TestChangedMarkdownPrintsIdenticalBytes covers determinism.
//
// Goal: the same input prints the same markdown on a second run and with one
// scheduler thread.
func TestChangedMarkdownPrintsIdenticalBytes(t *testing.T) {
	c := fixture(t, "new-package")
	first := c.changed("--format", "markdown")
	require.Equal(t, cli.ExitClean, first.code)
	assert.Equal(t, first.stdout, c.changed("--format", "markdown").stdout)

	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	assert.Equal(t, first.stdout, c.changed("--format", "markdown").stdout)
}

// TestChangedMarkdownBoldsOnlyWhatChanged covers the counts line.
//
// Goal: each non-zero count is bold and each zero count is plain.
func TestChangedMarkdownBoldsOnlyWhatChanged(t *testing.T) {
	got := fixture(t, "call-removed").changed("--format", "markdown")
	require.Equal(t, cli.ExitClean, got.code)
	assert.Contains(t, got.stdout, "\nCalls added: 0 · **Calls removed: 1**\n")
}

// bulkSource is a package of count functions that each call one outside function.
func bulkSource(pkg string, count int) string {
	var src strings.Builder
	fmt.Fprintf(&src,
		"package %s\n\nimport \"example.com/fakedb\"\n\nvar pool *fakedb.Pool\n\n", pkg)
	for i := range count {
		fmt.Fprintf(&src, "func Call%05d() { _ = pool.Exec(\"x\") }\n\n", i)
	}
	return src.String()
}

const (
	// Each call prints about 48 characters, so fitsCalls stays under the
	// limit and spillsCalls passes it alone, in a list or in a new package.
	fitsCalls   = 600
	spillsCalls = 1800
)

// bulkSize is how many calls the branch adds to an existing package and to a new one.
type bulkSize struct {
	existing int
	fresh    int
}

// bulkCheckout is a branch that adds count calls to an existing package, and
// optionally a new package with the same number of calls.
func bulkCheckout(t *testing.T, bulk bulkSize) checkout {
	t.Helper()
	c := newCheckout(t)
	c.write("go.mod", shopModule)
	c.copyFixture("fakedb", "fakedb")
	c.write("web/web.go", bulkSource("web", 1))
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.write("web/web.go", bulkSource("web", 1+bulk.existing))
	if bulk.fresh > 0 {
		c.write("bulk/bulk.go", bulkSource("bulk", bulk.fresh))
	}
	c.commit("head")
	return c
}

// TestChangedMarkdownStaysUnderTheCommentLimit covers the size cap.
//
// Goal: a report that fits is untouched, a report that passes 60,000
// characters drops the new package's calls first, then cuts the main list at a
// whole function and says how many calls were not shown, and the counts line
// keeps the full totals.
func TestChangedMarkdownStaysUnderTheCommentLimit(t *testing.T) {
	t.Run("fits", func(t *testing.T) {
		t.Parallel()
		got := bulkCheckout(t, bulkSize{existing: fitsCalls}).changed("--format", "markdown")
		require.Equal(t, cli.ExitClean, got.code)
		assert.LessOrEqual(t, utf8.RuneCountInString(got.stdout), commentLimit)
		assert.NotContains(t, got.stdout, "not shown")
		assert.Contains(t, got.stdout, "+   example.com/fakedb.Pool.Exec\n```\n")
		assert.Contains(t, got.stdout, fmt.Sprintf("**Calls added: %d**", fitsCalls))
	})

	t.Run("dropsNewPackageCallsFirst", func(t *testing.T) {
		t.Parallel()
		got := bulkCheckout(t, bulkSize{existing: 100, fresh: spillsCalls}).
			changed("--format", "markdown")
		require.Equal(t, cli.ExitClean, got.code)
		assert.LessOrEqual(t, utf8.RuneCountInString(got.stdout), commentLimit)
		assert.Contains(t, got.stdout,
			"Too long for a comment; run `tiger changed main` for every call.\n")
		assert.NotContains(t, got.stdout, "<details>")
		assert.NotContains(t, got.stdout, "not shown")
		assert.Contains(t, got.stdout, "- Uses outside packages: `example.com/fakedb` (Pool)\n")
		assert.Contains(t, got.stdout, "**Calls added: 100**")
		assert.Contains(t, got.stdout, fmt.Sprintf(
			"**New package `bulk`** — 1800 types and functions, %d calls:", spillsCalls))
	})

	t.Run("cutsTheMainListAtAWholeFunction", func(t *testing.T) {
		t.Parallel()
		const total = spillsCalls * 2
		got := bulkCheckout(t, bulkSize{existing: total}).changed("--format", "markdown")
		require.Equal(t, cli.ExitClean, got.code)
		out := got.stdout
		assert.LessOrEqual(t, utf8.RuneCountInString(out), commentLimit)
		assert.Contains(t, out, fmt.Sprintf("**Calls added: %d**", total))

		shown := strings.Count(out, "\n+   example.com/fakedb.Pool.Exec")
		hidden := total - shown
		require.Positive(t, hidden)
		assert.Contains(t, out, fmt.Sprintf(
			"```\n\n%d more calls in %d functions not shown; "+
				"run `tiger changed main` for the rest.\n",
			hidden, hidden))

		// Every shown header keeps its one call line, and the footer survives.
		assert.Equal(t, shown, strings.Count(out, "\nweb.Call"))
		assert.True(t, strings.HasSuffix(out,
			"A call moved into a new helper, or a renamed type whose calls changed, "+
				"shows as removed plus added.</sub>\n"))
		assert.Equal(t, 1, strings.Count(out, "<!-- tiger-changed -->"))
	})
}

// TestChangedMarkdownQuotesABaseRefWithBackticks covers a ref that would end a code span.
//
// Goal: a base ref containing a backtick sits inside a code span whose
// delimiter is longer than any backtick run in it.
func TestChangedMarkdownQuotesABaseRefWithBackticks(t *testing.T) {
	c := fixture(t, "call-added")
	c.git("branch", "release`candidate", "main")

	got := run(t, "changed", "-C", c.dir, "--format", "markdown", "release`candidate")
	require.Equal(t, cli.ExitClean, got.code)
	assert.Contains(t, got.stdout, "#### tiger changed: `` release`candidate `` (")
}

// TestChangedMarkdownSuggestsACommandAShellCanRun covers the pointer to the terminal.
//
// Goal: when the comment is cut, the suggested command quotes a base ref that
// a shell would read as syntax.
func TestChangedMarkdownSuggestsACommandAShellCanRun(t *testing.T) {
	c := bulkCheckout(t, bulkSize{existing: 100, fresh: spillsCalls})
	c.git("branch", "it's`odd", "main")

	got := run(t, "changed", "-C", c.dir, "--format", "markdown", "it's`odd")
	require.Equal(t, cli.ExitClean, got.code)
	assert.Contains(t, got.stdout, "run `` tiger changed 'it'\\''s`odd' `` for every call.\n")
}
