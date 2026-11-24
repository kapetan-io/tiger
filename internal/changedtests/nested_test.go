package changedtests_test

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// Goal: a nested go.mod the run has no permission to read is named in both
// the text and the JSON report, and its tests are not reported.
func TestChangedTestsNamesAModuleWhoseGoModCannotBeOpened(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	c.remove("tools/lint/lint_test.go")
	goMod := filepath.Join(c.dir, "tools", "go.mod")
	require.NoError(t, os.Chmod(goMod, 0o000))
	t.Cleanup(func() { require.NoError(t, os.Chmod(goMod, 0o644)) })

	doc := c.treeReport()

	assert.False(t, doc.ChangesExistingTests)
	assert.Empty(t, doc.Tests)
	assert.Equal(t, []unread{
		{Dir: "tools", Reason: "cannot read go.mod: permission denied at head"},
	}, doc.UnreadModules)

	got := run(t, "changed-tests", "-C", c.dir, "main")

	assert.Equal(t, command.ExitClean, got.code)
	assert.Contains(t, got.stdout, "Not read: the module in tools, so its tests are not "+
		"compared (cannot read go.mod: permission denied at head).\n")
}

// Goal: a go.mod that has no module line only at the merge base is named
// for the merge base alone.
func TestChangedTestsNamesAModuleUnreadOnlyAtTheMergeBase(t *testing.T) {
	c := newCheckout(t)
	c.write("tools/go.mod", "go 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.remove("tools/lint/lint_test.go")

	doc := c.treeReport()

	assert.False(t, doc.ChangesExistingTests)
	assert.Empty(t, doc.Tests)
	assert.Equal(t, []unread{
		{Dir: "tools", Reason: "go.mod has no module line at the merge base"},
	}, doc.UnreadModules)
}

// Goal: a module that is read is reported even when the module around it
// is unread, since its own go.mod makes it a separate module.
func TestChangedTestsReportsAModuleNestedInsideAnUnreadModule(t *testing.T) {
	c := newCheckout(t)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/gen/go.mod", "module example.com/gen\n\ngo 1.26\n")
	c.write("tools/gen/gen_test.go",
		"package gen\n\nimport \"testing\"\n\nfunc TestGen(t *testing.T) {}\n")
	c.branch()
	c.write("tools/go.mod", "go 1.26\n")
	c.remove("tools/gen/gen_test.go")

	doc := c.treeReport()

	assert.True(t, doc.ChangesExistingTests)
	assert.Equal(t, []test{{
		Package: "example.com/gen", File: "tools/gen/gen_test.go",
		Function: "TestGen", Change: "deleted",
	}}, doc.Tests)
	assert.Equal(t, []unread{
		{Dir: "tools", Reason: "go.mod has no module line at head"},
	}, doc.UnreadModules)
}

// Goal: a run from a subdirectory of the root module reads the nested
// modules below it, with files relative to the run directory.
func TestChangedTestsReadsNestedModulesBelowARunSubdirectory(t *testing.T) {
	c := newCheckout(t)
	c.write("internal/cart/cart_test.go", cartTest)
	c.write("internal/tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("internal/tools/lint/lint_test.go", lintTest)
	c.branch()
	c.remove("internal/cart/cart_test.go")
	c.remove("internal/tools/lint/lint_test.go")

	got := run(t, "changed-tests", "-C", filepath.Join(c.dir, "internal"),
		"--format", "json", "main")

	require.Equal(t, command.ExitClean, got.code, got.stderr)
	var doc treeReport
	require.NoError(t, json.Unmarshal([]byte(got.stdout), &doc))
	assert.Equal(t, []test{
		{
			Package: "example.com/shop/internal/cart", File: "cart/cart_test.go",
			Function: "TestEmpty", Change: "deleted",
		},
		{
			Package: "example.com/shop/internal/cart", File: "cart/cart_test.go",
			Function: "TestTotal", Change: "deleted",
		},
		{
			Package: "example.com/tools/lint", File: "tools/lint/lint_test.go",
			Function: "TestLint", Change: "deleted",
		},
	}, doc.Tests)
	assert.Equal(t, []unread{}, doc.UnreadModules)
}

// Goal: a run from a directory inside a nested module reports that
// module's tests under its own module path, and none of the root module's.
func TestChangedTestsReadsFromInsideANestedModule(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	c.remove("cart/cart_test.go")
	c.remove("tools/lint/lint_test.go")

	got := run(t, "changed-tests", "-C", filepath.Join(c.dir, "tools", "lint"),
		"--format", "json", "main")

	require.Equal(t, command.ExitClean, got.code, got.stderr)
	var doc treeReport
	require.NoError(t, json.Unmarshal([]byte(got.stdout), &doc))
	assert.Equal(t, []test{{
		Package: "example.com/tools/lint", File: "lint_test.go",
		Function: "TestLint", Change: "deleted",
	}}, doc.Tests)
}

// Goal: -C through a symbolic link to the checkout reads its nested
// modules, with files relative to the link.
func TestChangedTestsReadsNestedModulesThroughASymlinkedDirectory(t *testing.T) {
	c := newCheckout(t)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	c.remove("tools/lint/lint_test.go")
	link := filepath.Join(t.TempDir(), "shop")
	require.NoError(t, os.Symlink(c.dir, link))

	got := run(t, "changed-tests", "-C", link, "--format", "json", "main")

	require.Equal(t, command.ExitClean, got.code, got.stderr)
	var doc treeReport
	require.NoError(t, json.Unmarshal([]byte(got.stdout), &doc))
	assert.Equal(t, []test{{
		Package: "example.com/tools/lint", File: "tools/lint/lint_test.go",
		Function: "TestLint", Change: "deleted",
	}}, doc.Tests)
}

// Goal: a module under testdata, vendor, or a directory whose name starts
// with a dot or underscore is skipped, as go test skips it when expanding a
// package pattern, so test fixtures that hold a go.mod never fill the report.
func TestChangedTestsSkipsModulesInDirectoriesGoNeverBuilds(t *testing.T) {
	for _, dir := range []string{"testdata", "vendor", ".tools", "_tools"} {
		t.Run(dir, func(t *testing.T) {
			c := newCheckout(t)
			c.write(repoPath(dir+"/go.mod"), "module example.com/tools\n\ngo 1.26\n")
			c.write(repoPath(dir+"/lint/lint_test.go"), lintTest)
			c.branch()
			c.remove(repoPath(dir + "/lint/lint_test.go"))

			doc := c.treeReport()

			assert.False(t, doc.ChangesExistingTests)
			assert.Empty(t, doc.Tests)
			assert.Equal(t, []unread{}, doc.UnreadModules)
		})
	}
}

// Goal: the merge base is read from a temporary directory, and a go.mod
// above that directory belongs to neither revision, so it never adds tests
// or stops the report.
func TestChangedTestsIgnoresAGoModAboveTheMergeBaseCopy(t *testing.T) {
	for _, tc := range []struct {
		name  string
		goMod string
	}{
		{"Readable", "module example.com/outside\n\ngo 1.26\n"},
		{"NoModuleLine", "go 1.26\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCheckout(t)
			c.remove("go.mod")
			c.write("cart/cart_test.go", cartTest)
			c.branch()
			c.write("go.mod", "module example.com/shop\n\ngo 1.26\n")
			c.remove("cart/cart_test.go")
			outside := t.TempDir()
			goMod := filepath.Join(outside, "go.mod")
			require.NoError(t, os.WriteFile(goMod, []byte(tc.goMod), 0o644))
			require.NoError(t, os.Mkdir(filepath.Join(outside, "tmp"), 0o755))
			t.Setenv("TMPDIR", filepath.Join(outside, "tmp"))

			doc := c.treeReport()

			assert.False(t, doc.ChangesExistingTests)
			assert.Empty(t, doc.Tests)
		})
	}
}

// Goal: a nested module in a one-letter directory is still the module its
// tests belong to, so they are left out when its go.mod is unread.
func TestChangedTestsLeavesOutTestsOfAnUnreadOneLetterModule(t *testing.T) {
	c := newCheckout(t)
	c.write("a/go.mod", "module example.com/a\n\ngo 1.26\n")
	c.write("a/x_test.go", "package a\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n")
	c.branch()
	c.write("a/go.mod", "go 1.26\n")

	doc := c.treeReport()

	assert.False(t, doc.ChangesExistingTests)
	assert.Empty(t, doc.Tests)
	assert.Equal(t, []unread{{Dir: "a", Reason: "go.mod has no module line at head"}},
		doc.UnreadModules)
}

// Goal: a module read at the merge base that falls inside an unread module
// at head is not compared, since its tests at head were never read.
func TestChangedTestsLeavesOutAModuleSwallowedByAnUnreadModule(t *testing.T) {
	c := newCheckout(t)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/sub/go.mod", "module example.com/sub\n\ngo 1.26\n")
	c.write("tools/sub/x_test.go",
		"package sub\n\nimport \"testing\"\n\nfunc TestS(t *testing.T) {}\n")
	c.branch()
	c.remove("tools/sub/go.mod")
	c.write("tools/go.mod", "go 1.26\n")

	doc := c.treeReport()

	assert.False(t, doc.ChangesExistingTests)
	assert.Empty(t, doc.Tests)
	assert.Equal(t, []unread{{Dir: "tools", Reason: "go.mod has no module line at head"}},
		doc.UnreadModules)
}

// Goal: a nested module with a test file that does not parse is named with
// the parse error and left out, and the rest of the tree is still compared.
func TestChangedTestsNamesANestedModuleWhoseTestDoesNotParse(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.write("tools/gen/go.mod", "module example.com/gen\n\ngo 1.26\n")
	c.write("tools/gen/gen_test.go",
		"package gen\n\nimport \"testing\"\n\nfunc TestGen(t *testing.T) {}\n")
	c.branch()
	c.write("tools/lint/lint_test.go", "package lint\n\nfunc {\n")
	c.remove("tools/gen/gen_test.go")
	c.remove("cart/cart_test.go")

	doc := c.treeReport()

	assert.Equal(t, []test{
		{
			Package: "example.com/gen", File: "tools/gen/gen_test.go",
			Function: "TestGen", Change: "deleted",
		},
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestEmpty", Change: "deleted",
		},
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestTotal", Change: "deleted",
		},
	}, doc.Tests)
	assert.Equal(t, []unread{{
		Dir:    "tools",
		Reason: "cannot parse tools/lint/lint_test.go:3:6: expected 'IDENT', found '{' at head",
	}}, doc.UnreadModules)
}

// Goal: a test file in the run's own module that does not parse still
// produces no report, the same as before nested modules were read.
func TestChangedTestsExitsTwoWhenARunModuleTestDoesNotParse(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("cart/cart_test.go", "package cart\n\nfunc {\n")

	got := run(t, "changed-tests", "-C", c.dir, "main")

	assert.Equal(t, command.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "cannot parse cart/cart_test.go")
}

// Goal: a nested module with a test file the run has no permission to read
// is named with the file's path in the repository, not the absolute path.
func TestChangedTestsNamesANestedModuleWhoseTestCannotBeOpened(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	lint := filepath.Join(c.dir, "tools", "lint", "lint_test.go")
	require.NoError(t, os.Chmod(lint, 0o000))
	t.Cleanup(func() { require.NoError(t, os.Chmod(lint, 0o644)) })

	doc := c.treeReport()

	assert.Empty(t, doc.Tests)
	assert.Equal(t, []unread{{
		Dir:    "tools",
		Reason: "cannot read tools/lint/lint_test.go: permission denied at head",
	}}, doc.UnreadModules)
}

// Goal: a repository with only a root module gets an empty unread_modules
// list in the JSON, never a missing or null field.
func TestChangedTestsListsNoUnreadModulesForARootOnlyRepository(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()

	got := run(t, "changed-tests", "-C", c.dir, "--format", "json", "main")

	require.Equal(t, command.ExitClean, got.code, got.stderr)
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(got.stdout), &doc))
	assert.JSONEq(t, `[]`, string(doc["unread_modules"]))
}

// Goal: a nested module holding a directory the run cannot list is named,
// and the rest of the tree is still compared.
func TestChangedTestsNamesANestedModuleWithADirectoryThatCannotBeListed(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", lintTest)
	c.branch()
	c.remove("cart/cart_test.go")
	lint := filepath.Join(c.dir, "tools", "lint")
	require.NoError(t, os.Chmod(lint, 0o000))
	t.Cleanup(func() { require.NoError(t, os.Chmod(lint, 0o755)) })

	doc := c.treeReport()

	assert.Equal(t, []test{
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestEmpty", Change: "deleted",
		},
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestTotal", Change: "deleted",
		},
	}, doc.Tests)
	assert.Equal(t, []unread{{
		Dir:    "tools",
		Reason: "cannot read tools/lint: permission denied at head",
	}}, doc.UnreadModules)
}

// Goal: a directory the run cannot list inside the run's own module still
// produces no report, since that module's tests are not all known.
func TestChangedTestsExitsTwoWhenARunModuleDirectoryCannotBeListed(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	cart := filepath.Join(c.dir, "cart")
	require.NoError(t, os.Chmod(cart, 0o000))
	t.Cleanup(func() { require.NoError(t, os.Chmod(cart, 0o755)) })

	got := run(t, "changed-tests", "-C", c.dir, "main")

	assert.Equal(t, command.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "permission denied")
}
