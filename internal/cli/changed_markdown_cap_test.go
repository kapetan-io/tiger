package cli_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// TestChangedMarkdownCutsTheMainListAfterDroppingNewPackageCalls covers both
// cap steps in one comment.
//
// Goal: when dropping a new package's calls is not enough, the main list is
// cut as well, and the package keeps its heading and summary bullets.
func TestChangedMarkdownCutsTheMainListAfterDroppingNewPackageCalls(t *testing.T) {
	got := bulkCheckout(t, bulkSize{existing: 3600, fresh: 50}).changed("--format", "markdown")
	require.Equal(t, cli.ExitClean, got.code)
	assert.LessOrEqual(t, utf8.RuneCountInString(got.stdout), commentLimit)
	assert.Contains(t, got.stdout,
		"Too long for a comment; run `tiger changed main` for every call.\n")
	assert.Contains(t, got.stdout, " more calls in ")
	assert.NotContains(t, got.stdout, "<details>")
	assert.Contains(t, got.stdout, "**New package `bulk`**")
	assert.Contains(t, got.stdout, "- Uses outside packages: `example.com/fakedb` (Pool)\n")
}

// TestChangedMarkdownCountsCharactersNotBytes covers the unit of the cap.
//
// Goal: names of multi-byte characters that total over 60,000 bytes but under
// 60,000 characters are not cut.
func TestChangedMarkdownCountsCharactersNotBytes(t *testing.T) {
	var src strings.Builder
	src.WriteString("package web\n\nimport \"example.com/fakedb\"\n\nvar pool *fakedb.Pool\n\n")
	for i := range 900 {
		fmt.Fprintf(&src, "func 世界世界世界世界世界世界世界世界世界%05d() { _ = pool.Exec(\"x\") }\n\n", i)
	}
	c := newCheckout(t)
	c.write("go.mod", shopModule)
	c.copyFixture("fakedb", "fakedb")
	c.write("web/web.go", bulkSource("web", 1))
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.write("web/web.go", src.String())
	c.commit("head")

	got := c.changed("--format", "markdown")
	require.Equal(t, cli.ExitClean, got.code)
	assert.Greater(t, len(got.stdout), commentLimit)
	assert.LessOrEqual(t, utf8.RuneCountInString(got.stdout), commentLimit)
	assert.NotContains(t, got.stdout, "not shown")
}

// TestChangedMarkdownNamesTheMergeBase covers the title when main moved on.
//
// Goal: the title names the merge base, not the tip of the base ref.
func TestChangedMarkdownNamesTheMergeBase(t *testing.T) {
	c := fixture(t, "call-added")
	mergeBase := c.short("main")
	c.git("checkout", "-q", "main")
	c.write("README.md", "main moves on\n")
	c.commit("main moves")
	c.git("checkout", "-q", "feature")

	got := c.changed("--format", "markdown")
	require.Equal(t, cli.ExitClean, got.code)
	assert.Contains(t, got.stdout,
		"#### tiger changed: `main` (merge base "+mergeBase+") → `"+c.short("HEAD")+"`\n")
}
