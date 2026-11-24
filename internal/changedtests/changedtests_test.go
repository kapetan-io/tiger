package changedtests_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
	"github.com/kapetan-io/tiger/internal/command"
)

type outcome struct {
	code   int
	stdout string
	stderr string
}

func run(t *testing.T, args ...string) outcome {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := cli.Run(args, command.Streams{Stdout: stdout, Stderr: stderr})
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// checkout is a real git repository in a temporary directory, with pinned
// authors and dates so commit hashes are the same on every machine.
type checkout struct {
	t   *testing.T
	dir string
}

func newCheckout(t *testing.T) checkout {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	c := checkout{t: t, dir: dir}
	c.git("init", "-q", "-b", "main")
	c.write("go.mod", "module example.com/shop\n\ngo 1.26\n")
	return c
}

func (c checkout) git(args ...string) string {
	c.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = c.dir
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(entry string) bool {
		return strings.HasPrefix(entry, "GIT_")
	}),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(c.t, err, string(out))
	return strings.TrimSpace(string(out))
}

// repoPath is a slash path inside the checkout.
type repoPath string

func (c checkout) write(name repoPath, content string) {
	c.t.Helper()
	path := filepath.Join(c.dir, filepath.FromSlash(string(name)))
	require.NoError(c.t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(c.t, os.WriteFile(path, []byte(content), 0o644))
}

func (c checkout) remove(name repoPath) {
	c.t.Helper()
	require.NoError(c.t, os.Remove(filepath.Join(c.dir, filepath.FromSlash(string(name)))))
}

func (c checkout) commit(message string) {
	c.t.Helper()
	c.git("add", "-A")
	c.git("commit", "-q", "-m", message)
}

// branch commits the base on main and moves to a feature branch, so every
// later write is the change under report.
func (c checkout) branch() {
	c.t.Helper()
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
}

type report struct {
	Schema               string `json:"schema"`
	ChangesExistingTests bool   `json:"changes_existing_tests"`
	Tests                []test `json:"tests"`
}

type test struct {
	Package  string `json:"package"`
	File     string `json:"file"`
	Function string `json:"function"`
	Change   string `json:"change"`
}

func (c checkout) report() report {
	c.t.Helper()
	got := run(c.t, "changed-tests", "-C", c.dir, "--format", "json", "main")
	require.Equal(c.t, command.ExitClean, got.code, got.stderr)
	var doc report
	require.NoError(c.t, json.Unmarshal([]byte(got.stdout), &doc), got.stdout)
	return doc
}

const cartTest = `package cart

import "testing"

func TestTotal(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("bad")
	}
}

func TestEmpty(t *testing.T) {}
`

// Goal: a branch that edits one existing test and deletes another reports
// both, with package and file, and changes_existing_tests is true.
func TestChangedTestsReportsModifiedAndDeletedTests(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("cart/cart_test.go", strings.Replace(
		strings.Replace(cartTest, "1+1 != 2", "1+2 != 3", 1),
		"func TestEmpty(t *testing.T) {}\n", "", 1))

	doc := c.report()

	assert.Equal(t, "tiger.changed-tests/v1", doc.Schema)
	assert.True(t, doc.ChangesExistingTests)
	assert.Equal(t, []test{
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestEmpty", Change: "deleted",
		},
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestTotal", Change: "modified",
		},
	}, doc.Tests)
}

// Goal: the text report is byte-for-byte what an agent parses: title, counts,
// one line per test under its package, then what was compared.
func TestChangedTestsTextReport(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("cart/cart_test.go", strings.Replace(cartTest, "1+1 != 2", "1+2 != 3", 1))
	c.commit("edit")
	head := c.git("rev-parse", "--short=7", "HEAD")
	base := c.git("rev-parse", "--short=7", "main")

	got := run(t, "changed-tests", "-C", c.dir, "main")

	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "tiger changed-tests: main ("+base+") -> "+head+"\n"+
		"Test functions that existed at the merge base and that this change modifies "+
		"or deletes. Each one guarded behavior that may now be different. "+
		"Check that each change is intended.\n\n"+
		"Modified: 1 · Deleted: 0\n\n"+
		"example.com/shop/cart\n"+
		"  modified  TestTotal  cart/cart_test.go\n\n"+
		"Compared 2 test functions in 1 package at the merge base.\n"+
		"Counted only top-level Test, Benchmark, Fuzz and Example functions in "+
		"_test.go files. Tests added since the merge base, test methods on suite "+
		"types, helpers and testdata are not listed, and an edit to comments or "+
		"spacing alone is not a change, except an Example's Output comment.\n",
		got.stdout)
}

// Goal: a branch that only adds tests, in an existing file and in a new
// package, reports no changed existing tests.
func TestChangedTestsIgnoresAddedTests(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("cart/cart_test.go", cartTest+"\nfunc TestNew(t *testing.T) {}\n")
	c.write("pay/pay_test.go", "package pay\n\nimport \"testing\"\n\n"+
		"func TestPay(t *testing.T) {}\n")

	doc := c.report()

	assert.False(t, doc.ChangesExistingTests)
	assert.Empty(t, doc.Tests)
	assert.NotNil(t, doc.Tests, "tests is an empty list, not null")
}

// Goal: an edit to comments or layout alone does not count as a change.
func TestChangedTestsIgnoresCommentAndSpacingEdits(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("cart/cart_test.go", `package cart

import "testing"

// TestTotal now has a doc comment.
func TestTotal(t *testing.T) {
	// a new comment inside the body

	if 1+1 != 2 { t.Fatal("bad") } /* trailing */
}

func TestEmpty(t *testing.T) {
}
`)

	assert.Empty(t, c.report().Tests)
}

// Goal: a trailing comma that gofmt adds when a call spans lines is layout,
// so joining the call onto one line is not a change.
func TestChangedTestsIgnoresATrailingCommaOnAJoinedLine(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", "package cart\n\nimport \"testing\"\n\n"+
		"func TestJoin(t *testing.T) {\n\tt.Log(\n\t\t1,\n\t\t2,\n\t)\n}\n")
	c.branch()
	c.write("cart/cart_test.go", "package cart\n\nimport \"testing\"\n\n"+
		"func TestJoin(t *testing.T) {\n\tt.Log(1, 2)\n}\n")

	assert.Empty(t, c.report().Tests)
}

// Goal: moving a test between the package and its external test package in
// the same directory is a move, not a deletion; go test runs both together.
func TestChangedTestsFollowsATestIntoTheExternalTestPackage(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("cart/cart_test.go",
		strings.Replace(cartTest, "package cart\n", "package cart_test\n", 1))

	assert.Empty(t, c.report().Tests)
}

// Goal: a branch that renames the module reports the tests it edits under
// the new path, and does not report every unchanged test as deleted.
func TestChangedTestsFollowsARenamedModule(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("go.mod", "module example.com/store\n\ngo 1.26\n")
	c.write("cart/cart_test.go", strings.Replace(cartTest, "1+1 != 2", "1+2 != 3", 1))

	assert.Equal(t, []test{{
		Package: "example.com/store/cart", File: "cart/cart_test.go",
		Function: "TestTotal", Change: "modified",
	}}, c.report().Tests)
}

// Goal: -C through a symbolic link reads the module the link points into.
func TestChangedTestsRunsThroughASymlinkedDirectory(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.remove("cart/cart_test.go")
	link := filepath.Join(t.TempDir(), "cart")
	require.NoError(t, os.Symlink(filepath.Join(c.dir, "cart"), link))

	got := run(t, "changed-tests", "-C", link, "--format", "json", "main")

	require.Equal(t, command.ExitClean, got.code, got.stderr)
	var doc report
	require.NoError(t, json.Unmarshal([]byte(got.stdout), &doc))
	assert.Equal(t, []test{
		{
			Package: "example.com/shop/cart", File: "cart_test.go",
			Function: "TestEmpty", Change: "deleted",
		},
		{
			Package: "example.com/shop/cart", File: "cart_test.go",
			Function: "TestTotal", Change: "deleted",
		},
	}, doc.Tests)
}

// Goal: a test moved to another _test.go file of the same package with the
// same body is not a change; moved and edited is reported at its new file.
func TestChangedTestsFollowsATestMovedBetweenFiles(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.remove("cart/cart_test.go")
	c.write("cart/total_test.go", "package cart\n\nimport \"testing\"\n\n"+
		"func TestTotal(t *testing.T) {\n\tif 1+1 != 2 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	c.write("cart/empty_test.go", "package cart\n\nimport \"testing\"\n\n"+
		"func TestEmpty(t *testing.T) { t.Log(\"edited\") }\n")

	assert.Equal(t, []test{{
		Package: "example.com/shop/cart", File: "cart/empty_test.go",
		Function: "TestEmpty", Change: "modified",
	}}, c.report().Tests)
}

// Goal: a changed signature counts, and a test in an external test package
// is named by its directory's package, which go test builds it with.
func TestChangedTestsReportsASignatureChangeInAnExternalTestPackage(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/fuzz_test.go", "package cart_test\n\nimport \"testing\"\n\n"+
		"func FuzzTotal(f *testing.F) {}\n")
	c.branch()
	c.write("cart/fuzz_test.go", "package cart_test\n\nimport \"testing\"\n\n"+
		"func FuzzTotal(t *testing.T) {}\n")

	assert.Equal(t, []test{{
		Package: "example.com/shop/cart", File: "cart/fuzz_test.go",
		Function: "FuzzTotal", Change: "modified",
	}}, c.report().Tests)
}

// Goal: only top-level Test, Benchmark, Fuzz and Example functions count.
// Helpers, suite methods, look-alike names and testdata files are not
// reported however they change.
func TestChangedTestsCountsOnlyTestFunctions(t *testing.T) {
	c := newCheckout(t)
	before := `package cart

import "testing"

type suite struct{}

func (suite) TestMethod(t *testing.T) {}

func helper() int { return 1 }

func Testify() {}

func BenchmarkTotal(b *testing.B) {}

func Example() {}

func ExampleCart_total() {}
`
	c.write("cart/cart_test.go", before)
	c.write("cart/testdata/fixture_test.go", "package fixture\n\nfunc TestFixture() {}\n")
	c.write("cart/plain.go", "package cart\n\nfunc TestLike() {}\n")
	c.branch()
	c.write("cart/cart_test.go", "package cart\n")
	c.write("cart/testdata/fixture_test.go", "package fixture\n")
	c.write("cart/plain.go", "package cart\n")

	var names []string
	for _, got := range c.report().Tests {
		names = append(names, got.Function+" "+got.Change)
	}
	assert.Equal(t, []string{
		"BenchmarkTotal deleted", "Example deleted", "ExampleCart_total deleted",
	}, names)
}

// Goal: the comparison runs from the merge base to the working tree, so a
// test changed on main after the branch point is not reported and an
// uncommitted edit is.
func TestChangedTestsComparesTheMergeBaseWithTheWorkingTree(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.git("checkout", "-q", "main")
	c.write("cart/cart_test.go", strings.Replace(cartTest, "1+1 != 2", "2+2 != 4", 1))
	c.commit("main moves on")
	c.git("checkout", "-q", "feature")
	c.write("cart/cart_test.go", strings.Replace(cartTest,
		"func TestEmpty(t *testing.T) {}", "func TestEmpty(t *testing.T) { t.Skip() }", 1))

	assert.Equal(t, []test{{
		Package: "example.com/shop/cart", File: "cart/cart_test.go",
		Function: "TestEmpty", Change: "modified",
	}}, c.report().Tests)
}

// Goal: a run from a subdirectory of the module reports only the tests
// under it, with files relative to it, as tiger changed does.
func TestChangedTestsReadsFromTheRunDirectory(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("pay/pay_test.go",
		"package pay\n\nimport \"testing\"\n\nfunc TestPay(t *testing.T) {}\n")
	c.branch()
	c.remove("cart/cart_test.go")
	c.remove("pay/pay_test.go")

	got := run(t, "changed-tests", "-C", filepath.Join(c.dir, "pay"), "--format", "json", "main")

	require.Equal(t, command.ExitClean, got.code, got.stderr)
	var doc report
	require.NoError(t, json.Unmarshal([]byte(got.stdout), &doc))
	assert.Equal(t, []test{{
		Package: "example.com/shop/pay", File: "pay_test.go",
		Function: "TestPay", Change: "deleted",
	}}, doc.Tests)
}

// Goal: exit 2 with a reason on stderr when no report can be produced.
func TestChangedTestsExitsTwoWhenNoReportCanBeProduced(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	for _, tc := range []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			"NoBase",
			[]string{"-C", c.dir},
			"tiger changed-tests: expected a base revision, " +
				"as in: tiger changed-tests [-C dir] <base>\n",
		},
		{
			"UnknownBase",
			[]string{"-C", c.dir, "nope"},
			"tiger changed-tests: cannot find revision nope\n",
		},
		{
			"UnknownFormat",
			[]string{"-C", c.dir, "--format", "markdown", "main"},
			"tiger changed-tests: unknown format \"markdown\"; the formats are: text, json\n",
		},
		{
			"NotARepository",
			[]string{"-C", t.TempDir(), "main"},
			"tiger changed-tests: run it inside a git checkout, " +
				"because it compares against a git revision\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, append([]string{"changed-tests"}, tc.args...)...)
			assert.Equal(t, command.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Equal(t, tc.stderr, got.stderr)
		})
	}
}

// Goal: a test file that does not parse at head produces no report, so a
// test inside it is never mistaken for deleted.
func TestChangedTestsExitsTwoWhenATestFileDoesNotParse(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("cart/cart_test.go", "package cart\n\nfunc TestTotal( {\n")

	got := run(t, "changed-tests", "-C", c.dir, "main")

	assert.Equal(t, command.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "tiger changed-tests: cannot read the tests at head (")
	assert.Contains(t, got.stderr, "cannot parse cart/cart_test.go")
}

// Goal: a base revision with no go.mod has no tests, so a branch that adds
// the module and its tests reports no changed existing tests.
func TestChangedTestsReportsNothingWhenTheBaseHasNoModule(t *testing.T) {
	c := newCheckout(t)
	c.remove("go.mod")
	c.write("README.md", "shop\n")
	c.branch()
	c.write("go.mod", "module example.com/shop\n\ngo 1.26\n")
	c.write("cart/cart_test.go", cartTest)

	doc := c.report()

	assert.False(t, doc.ChangesExistingTests)
	assert.Empty(t, doc.Tests)
}

// Goal: tests in a file go never builds (//go:build ignore) are not read,
// a test whose file becomes ignored is reported deleted, because go test no
// longer runs it, and a test removed from a nested module is reported deleted.
func TestChangedTestsSkipsIgnoredFilesAndReadsNestedModules(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("cart/gen_test.go", "//go:build ignore\n\npackage cart\n\n"+
		"import \"testing\"\n\nfunc TestGen(t *testing.T) {}\n")
	c.write("tools/go.mod", "module example.com/tools\n\ngo 1.26\n")
	c.write("tools/lint/lint_test.go", "package lint\n\nimport \"testing\"\n\n"+
		"func TestLint(t *testing.T) {}\n")
	c.branch()
	c.write("cart/cart_test.go", "//go:build ignore\n\n"+cartTest)
	c.write("cart/gen_test.go", "//go:build ignore\n\npackage cart\n\n"+
		"import \"testing\"\n\nfunc TestGen(t *testing.T) { t.Skip() }\n")
	c.remove("tools/lint/lint_test.go")

	assert.Equal(t, []test{
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestEmpty", Change: "deleted",
		},
		{
			Package: "example.com/shop/cart", File: "cart/cart_test.go",
			Function: "TestTotal", Change: "deleted",
		},
		{
			Package: "example.com/tools/lint", File: "tools/lint/lint_test.go",
			Function: "TestLint", Change: "deleted",
		},
	}, c.report().Tests)
}

// Goal: a test defined once per build tag is compared across every
// definition, and the report names the file whose definition changed.
func TestChangedTestsComparesEveryBuildTagDefinition(t *testing.T) {
	const linux = "//go:build linux\n\npackage cart\n\nimport \"testing\"\n\n" +
		"func TestPath(t *testing.T) { t.Log(\"/\") }\n"
	const other = "//go:build !linux\n\npackage cart\n\nimport \"testing\"\n\n" +
		"func TestPath(t *testing.T) { t.Log(\"\\\\\") }\n"
	c := newCheckout(t)
	c.write("cart/path_linux_test.go", linux)
	c.write("cart/path_other_test.go", other)
	c.branch()
	c.write("cart/path_other_test.go", strings.Replace(other, "t.Log", "t.Skip", 1))

	assert.Equal(t, []test{{
		Package: "example.com/shop/cart", File: "cart/path_other_test.go",
		Function: "TestPath", Change: "modified",
	}}, c.report().Tests)
}

// Goal: the working tree, not the last commit, is what counts, so a
// committed edit that an uncommitted edit reverts is not a change.
func TestChangedTestsReadsTheDirtyWorkingTree(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.branch()
	c.write("cart/cart_test.go", strings.Replace(cartTest, "1+1 != 2", "1+2 != 3", 1))
	c.commit("edit")
	c.write("cart/cart_test.go", cartTest)

	assert.Empty(t, c.report().Tests)
}

// Goal: an Example's // Output: comment is what go test checks, so editing
// it changes the test even though it is a comment.
func TestChangedTestsReportsAnEditedExampleOutput(t *testing.T) {
	const example = "package cart_test\n\nimport \"fmt\"\n\n" +
		"func ExampleTotal() {\n\tfmt.Println(3)\n\t// Output: 3\n}\n"
	c := newCheckout(t)
	c.write("cart/example_test.go", example)
	c.branch()
	c.write("cart/example_test.go", strings.Replace(example, "// Output: 3", "// Output: 4", 1))

	assert.Equal(t, []test{{
		Package: "example.com/shop/cart", File: "cart/example_test.go",
		Function: "ExampleTotal", Change: "modified",
	}}, c.report().Tests)
}

// Goal: a package and its external test package may both declare TestTotal.
// Deleting one while the other stays reports a deletion at the deleted
// one's file, not a modification.
func TestChangedTestsReportsOneOfTwoSameNamedTestsDeleted(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/cart_test.go", cartTest)
	c.write("cart/external_test.go", "package cart_test\n\nimport \"testing\"\n\n"+
		"func TestTotal(t *testing.T) { t.Log(\"outside\") }\n")
	c.branch()
	c.remove("cart/external_test.go")

	assert.Equal(t, []test{{
		Package: "example.com/shop/cart", File: "cart/external_test.go",
		Function: "TestTotal", Change: "deleted",
	}}, c.report().Tests)
}

// tagged is a _test.go file declaring TestX under a build tag, with a body
// that differs by the value it assigns.
func tagged(tag string, value int) string {
	return fmt.Sprintf("//go:build %s\n\npackage cart\n\nimport \"testing\"\n\n"+
		"func TestX(t *testing.T) { _ = %d }\n", tag, value)
}

// Goal: when a test has one definition per build tag, the report names the
// file each change happened in: a deleted file, an edited file, never a
// file that is unchanged or newly added.
func TestChangedTestsNamesTheFileOfEachBuildTagDefinition(t *testing.T) {
	for _, test := range []struct {
		name  string
		base  map[repoPath]string
		head  map[repoPath]string
		wants []string
	}{
		{
			name: "DeletedTwinOfAnIdenticalDefinition",
			base: map[repoPath]string{
				"cart/a_test.go": tagged("linux", 1), "cart/b_test.go": tagged("!linux", 1),
			},
			head:  map[repoPath]string{"cart/a_test.go": ""},
			wants: []string{"deleted cart/a_test.go"},
		},
		{
			name: "EditNextToANewDefinition",
			base: map[repoPath]string{"cart/b_test.go": tagged("linux", 2)},
			head: map[repoPath]string{
				"cart/a_test.go": tagged("!linux", 9), "cart/b_test.go": tagged("linux", 3),
			},
			wants: []string{"modified cart/b_test.go"},
		},
		{
			name: "OneDeletedOneEdited",
			base: map[repoPath]string{
				"cart/a_test.go": tagged("linux", 1), "cart/b_test.go": tagged("!linux", 2),
			},
			head:  map[repoPath]string{"cart/a_test.go": "", "cart/b_test.go": tagged("!linux", 3)},
			wants: []string{"deleted cart/a_test.go", "modified cart/b_test.go"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := newCheckout(t)
			for _, name := range slices.Sorted(maps.Keys(test.base)) {
				c.write(name, test.base[name])
			}
			c.branch()
			for _, name := range slices.Sorted(maps.Keys(test.head)) {
				if test.head[name] == "" {
					c.remove(name)
					continue
				}
				c.write(name, test.head[name])
			}
			var got []string
			for _, row := range c.report().Tests {
				got = append(got, row.Change+" "+row.File)
			}
			assert.Equal(t, test.wants, got)
		})
	}
}

// Goal: switching an Example from "// Output:" to "// Unordered output:"
// with the same lines is a change, because go test then ignores line order.
func TestChangedTestsReportsAnExampleSwitchedToUnorderedOutput(t *testing.T) {
	const example = "package cart_test\n\nimport \"fmt\"\n\n" +
		"func ExampleTotal() {\n\tfmt.Println(3)\n\t// Output: 3\n}\n"
	c := newCheckout(t)
	c.write("cart/example_test.go", example)
	c.branch()
	c.write("cart/example_test.go",
		strings.Replace(example, "// Output: 3", "// Unordered output: 3", 1))

	assert.Equal(t, []test{{
		Package: "example.com/shop/cart", File: "cart/example_test.go",
		Function: "ExampleTotal", Change: "modified",
	}}, c.report().Tests)
}

// Goal: go test's naming rule decides what is a test. After the prefix comes
// nothing, or anything but a lowercase letter: Test_snake, Test2 and
// Example_suffix are tests, and Testing, Benchmarking and Examples are not.
func TestChangedTestsFollowsTheGoTestNameRule(t *testing.T) {
	c := newCheckout(t)
	c.write("cart/names_test.go", "package cart\n\nimport \"testing\"\n\n"+
		"func Test(t *testing.T) {}\n\n"+
		"func Test_snake(t *testing.T) {}\n\n"+
		"func Test2(t *testing.T) {}\n\n"+
		"func Example_suffix() {}\n\n"+
		"func Testing(t *testing.T) {}\n\n"+
		"func Benchmarking(b *testing.B) {}\n\n"+
		"func Examples() {}\n")
	c.branch()
	c.remove("cart/names_test.go")

	var names []string
	for _, got := range c.report().Tests {
		names = append(names, got.Function)
	}
	assert.Equal(t, []string{"Example_suffix", "Test", "Test2", "Test_snake"}, names)
}
