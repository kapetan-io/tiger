package changedtests_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
)

type treeReport struct {
	ChangesExistingTests bool     `json:"changes_existing_tests"`
	Tests                []test   `json:"tests"`
	UnreadModules        []unread `json:"unread_modules"`
}

type unread struct {
	Dir    string `json:"dir"`
	Reason string `json:"reason"`
}

func (c checkout) treeReport() treeReport {
	c.t.Helper()
	got := run(c.t, "changed-tests", "-C", c.dir, "--format", "json", "main")
	require.Equal(c.t, command.ExitClean, got.code, got.stderr)
	var doc treeReport
	require.NoError(c.t, json.Unmarshal([]byte(got.stdout), &doc), got.stdout)
	return doc
}

const lintTest = "package lint\n\nimport \"testing\"\n\nfunc TestLint(t *testing.T) {}\n"

// Goal: one run at the root reports a changed test in a nested module, and
// in a module nested inside that one, beside the root module's tests.
func TestChangedTestsReportsTestsInNestedModules(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.write("tools/gen/go.mod", "module example.com/gen\n\ngo 1.26\n")
	c.write("tools/gen/gen_test.go",
		"package gen\n\nimport \"testing\"\n\nfunc TestGen(t *testing.T) {}\n")
	c.branch()
	c.write("cart/cart_test.go", strings.Replace(cartTest, "1+1 != 2", "1+2 != 3", 1))
	c.write("tools/lint/lint_test.go", strings.Replace(lintTest, "{}", "{ t.Skip() }", 1))
	c.remove("tools/gen/gen_test.go")

	doc := c.treeReport()

	assert.True(t, doc.ChangesExistingTests)
	assert.Equal(t, []test{
		{
			Package: "example.com/gen", File: "tools/gen/gen_test.go",
			Function: "TestGen", Change: "deleted",
		},
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestTotal", Change: "modified",
		},
		{
			Package: "example.com/tools/lint", File: "tools/lint/lint_test.go",
			Function: "TestLint", Change: "modified",
		},
	}, doc.Tests)
	assert.Equal(t, []unread{}, doc.UnreadModules)
}

// Goal: changes_existing_tests is true when the only changed test is in a
// nested module, which is what a pull request labeler reads.
func TestChangedTestsFlagsAChangeOnlyInANestedModule(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	c.write("tools/lint/lint_test.go", strings.Replace(lintTest, "{}", "{ t.Skip() }", 1))

	doc := c.treeReport()

	assert.True(t, doc.ChangesExistingTests)
	assert.Equal(t, []test{{
		Package: "example.com/tools/lint", File: "tools/lint/lint_test.go",
		Function: "TestLint", Change: "modified",
	}}, doc.Tests)
}

// Goal: a branch that removes a whole nested module reports its tests as
// deleted.
func TestChangedTestsReportsARemovedNestedModule(t *testing.T) {
	c := newCheckout(t)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	c.remove("tools/lint/lint_test.go")
	c.remove("tools/go.mod")

	doc := c.treeReport()

	assert.Equal(t, []test{{
		Package: "example.com/tools/lint", File: "tools/lint/lint_test.go",
		Function: "TestLint", Change: "deleted",
	}}, doc.Tests)
}

// Goal: a repository whose root has no go.mod still reports the changed
// tests of the modules below it.
func TestChangedTestsReadsModulesBelowARootWithNoModule(t *testing.T) {
	c := newCheckout(t)
	c.remove("go.mod")
	c.write("api/go.mod", "module example.com/api\n\ngo 1.26\n")
	c.write("api/cart/cart_test.go", cartTest)
	c.branch()
	c.remove("api/cart/cart_test.go")

	doc := c.treeReport()

	assert.True(t, doc.ChangesExistingTests)
	assert.Equal(t, []test{
		{
			Package: "example.com/api/cart", File: "api/cart/cart_test.go",
			Function: "TestEmpty", Change: "deleted",
		},
		{
			Package: "example.com/api/cart", File: "api/cart/cart_test.go",
			Function: "TestTotal", Change: "deleted",
		},
	}, doc.Tests)
}

// Goal: a go.mod the run cannot read is named in the JSON with its reason
// and revision, so an empty list never reads as "nothing changed" there.
func TestChangedTestsNamesAnUnreadModule(t *testing.T) {
	c := newCheckout(t)
	c.write("broken/go.mod", "go 1.26\n")
	c.write("broken/x/x_test.go", lintTest)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	c.remove("broken/x/x_test.go")
	c.write("tools/go.mod", "go 1.26\n")
	c.remove("tools/lint/lint_test.go")

	doc := c.treeReport()

	assert.False(t, doc.ChangesExistingTests)
	assert.Empty(t, doc.Tests)
	assert.Equal(t, []unread{
		{Dir: "broken", Reason: "go.mod has no module line at head and the merge base"},
		{Dir: "tools", Reason: "go.mod has no module line at head"},
	}, doc.UnreadModules)
}

// Goal: the text report names an unread module on its own line after what
// was compared.
func TestChangedTestsTextNamesAnUnreadModule(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("tools/go.mod", "go 1.26\n")
	c.branch()

	got := run(t, "changed-tests", "-C", c.dir, "main")

	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout,
		"Compared 2 test functions in 1 package at the merge base.\n"+
			"Not read: the module in tools, so its tests are not compared "+
			"(go.mod has no module line at head and the merge base).\n")
}

// Goal: a run whose own go.mod cannot be read produces no report, the same
// as before nested modules were read, since none of its tests are known.
func TestChangedTestsExitsTwoWhenTheRunModuleIsUnread(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("go.mod", "go 1.26\n")

	got := run(t, "changed-tests", "-C", c.dir, "main")

	assert.Equal(t, command.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "cannot read the tests at head (")
	assert.Contains(t, got.stderr, "go.mod has no module line")
}
