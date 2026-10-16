package cli_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGapStep2KeepsFoldedNewPackage(t *testing.T) {
	got := bulkCheckout(t, bulkSize{existing: 3600, fresh: 50}).changed("--format", "markdown")
	out := got.stdout
	assert.LessOrEqual(t, utf8.RuneCountInString(out), commentLimit)
	assert.Contains(t, out, "Too long for a comment; run `tiger changed main` for every call.")
	assert.Contains(t, out, "more calls in")
	assert.NotContains(t, out, "<details>")
	assert.Contains(t, out, "**New package `bulk`**")
	assert.Contains(t, out, "- Uses outside packages:")
}

func TestGapCutLeavesZeroFunctions(t *testing.T) {
	c := newCheckout(t)
	c.write("go.mod", shopModule)
	c.copyFixture("fakedb", "fakedb")
	c.write("web/web.go", bulkSource("web", 1))
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	long := strings.Repeat("A", 61000)
	c.write("web/web.go", bulkSource("web", 1)+"func "+long+"() { _ = pool.Exec(\"x\") }\n")
	c.commit("head")
	out := c.changed("--format", "markdown").stdout
	assert.NotContains(t, out, "```diff")
	assert.Contains(t, out, "1 more calls in 1 functions not shown; run `tiger changed main` for the rest.")
	assert.LessOrEqual(t, utf8.RuneCountInString(out), commentLimit)
}

func TestGapCountsCharactersNotBytes(t *testing.T) {
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
	out := c.changed("--format", "markdown").stdout
	t.Log(utf8.RuneCountInString(out), len(out), strings.Contains(out,"not shown"))
	require.Greater(t, len(out), commentLimit)
	assert.NotContains(t, out, "not shown")
}

func TestGapMergeBaseDiffersTitle(t *testing.T) {
	c := fixture(t, "call-added")
	c.git("checkout", "-q", "main")
	c.write("README2.md", "x\n")
	c.commit("main moves")
	c.git("checkout", "-q", "-")
	out := c.changed("--format", "markdown").stdout
	t.Log(strings.Split(out, "\n")[1])
}
