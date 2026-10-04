package changed_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"changed/changed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goMod = `module example.com/shop

go 1.26

require example.com/fakedb v0.0.0

replace example.com/fakedb => ./fakedb
`

// repo builds a git repository with two commits: testdata/<fixture>/base,
// then testdata/<fixture>/head. Both sit beside testdata/fakedb, a separate
// module that imports os and so passes the syscall filter like a real driver.
func repo(t *testing.T, fixture string) string {
	// t.TempDir is under /var on macOS, a symlink to /private/var; git
	// reports the resolved path, so compare against the resolved one.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644))
	require.NoError(t, os.CopyFS(filepath.Join(dir, "fakedb"), os.DirFS("testdata/fakedb")))
	require.NoError(t, os.CopyFS(dir, os.DirFS(filepath.Join("testdata", fixture, "base"))))
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "base")

	entries, err := os.ReadDir(filepath.Join("testdata", fixture, "base"))
	require.NoError(t, err)
	for _, entry := range entries {
		require.NoError(t, os.RemoveAll(filepath.Join(dir, entry.Name())))
	}
	require.NoError(t, os.CopyFS(dir, os.DirFS(filepath.Join("testdata", fixture, "head"))))
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "head")
	return dir
}

func git(t *testing.T, dir string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

// report compares the fixture's head commit, checked out on disk, against
// the commit before it, the way `changed -base HEAD~1` does.
func report(t *testing.T, fixture string, filter changed.Filter) string {
	dir := repo(t, fixture)
	base, err := changed.LoadRevision(dir, "HEAD~1", filter)
	require.NoError(t, err)
	head, err := changed.Load(dir, filter)
	require.NoError(t, err)
	return changed.Report(changed.Diff(base, head))
}

// An HTTP handler that starts writing to the database itself is reported on
// the handler, naming the driver's type and the method that made the call.
func TestHandlerStartsCallingDatabase(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "web.Handler\n"+
		"  + example.com/fakedb.Pool: Greet calls Exec\n",
		report(t, "handler", changed.ByPackage))
}

// When Billing starts calling the database, the report names Billing only.
// Handler calls Billing and did not change, so nothing is said about it.
func TestReachIsNotPropagatedToCallers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "billing.Billing\n"+
		"  + example.com/fakedb.Pool: Charge calls Exec\n",
		report(t, "propagation", changed.ByPackage))
}

// A new call through a module interface is named by the interface, not by
// the implementation that happens to reach the database.
func TestInterfaceCallNamesTheInterface(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "web.Handler\n"+
		"  + store.Users: Greet calls Find\n",
		report(t, "interface", changed.ByPackage))
}

// strings, sort and strconv never import syscall, so new calls into them
// produce no report.
func TestPureHelpersAreSilent(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", report(t, "pure", changed.ByPackage))
}

// A call that goes away is reported with a minus, naming the call the base
// made.
func TestRemovedCall(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "web.Handler\n"+
		"  - example.com/fakedb.Pool: Greet calls Exec\n",
		report(t, "removed", changed.ByPackage))
}

// Moving a call into an unexported method and a closure of the same type
// keeps the owner and the target, so the edge is unchanged and nothing is
// reported.
func TestRefactorWithinOwnerIsSilent(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", report(t, "method-refactor", changed.ByPackage))
}

// A free function is its own owner, so moving a call from a method into an
// unexported free helper reads as the method dropping the edge and the
// helper gaining it.
func TestRefactorIntoFreeHelperMovesTheEdge(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "web.Handler\n"+
		"  - example.com/fakedb.Pool: Greet calls Exec\n"+
		"web.record\n"+
		"  + example.com/fakedb.Pool: record calls Exec\n",
		report(t, "helper-refactor", changed.ByPackage))
}

// fmt imports os, so fmt.Errorf and fmt.Sprintf leak through the package
// filter; errors does not reach syscall and stays silent.
func TestFmtLeaksThroughPackageFilter(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "web.Handler\n"+
		"  + fmt.Errorf: Greet calls Errorf\n"+
		"  + fmt.Sprintf: Greet calls Sprintf\n",
		report(t, "fmt-errors", changed.ByPackage))
}

// context imports time, which imports syscall, so a new wait on ctx.Done
// is reported as an edge to context.Context.
func TestContextIsReported(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "web.Handler\n"+
		"  + context.Context: Greet calls Done; Greet calls Err\n",
		report(t, "context", changed.ByPackage))
}

// io does not import syscall, so a new write through an io.Writer field is
// not reported, even though the writer may be a file or a socket.
func TestWriteThroughIOWriterIsSilent(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", report(t, "io-writer", changed.ByPackage))
}

// The handler starts sending queries to a store worker over a channel. A
// channel send is not a call, so the report is silent although the handler
// now drives database writes.
func TestMessagePassingIsSilent(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", report(t, "channel", changed.ByPackage))
}

// The worker at the receiving end of the channel starts writing to the
// database. The report names the worker, so a change routed through a channel
// still shows on the struct where the new call was written.
func TestChannelReceiverIsReported(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "store.Writer\n"+
		"  + example.com/fakedb.Pool: Run calls Exec\n",
		report(t, "channel-receiver", changed.ByPackage))
}

// Retry starts calling another method on a type it already calls. That is
// not a surprising change, so nothing is reported.
func TestNewMethodOnKnownTypeIsSilent(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", report(t, "same-target", changed.ByPackage))
}

// Retry starts opening its own database connection. The constructor is a new
// call for the struct, so it is reported although the struct already used the
// database through its pool.
func TestNewConstructorIsReported(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "store.Queue\n"+
		"  + example.com/fakedb.Open: Retry calls Open\n",
		report(t, "new-constructor", changed.ByPackage))
}

// The function filter keeps an outside call only when its body statically
// reaches syscall. It drops fmt, but it also drops os.File.Close behind
// fakedb.Pool.Close, which reaches the close system call through a package
// variable rather than a static call.
func TestFunctionFilter(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		fixture string
		want    string
	}{
		{
			fixture: "handler",
			want: "web.Handler\n" +
				"  + example.com/fakedb.Pool: Greet calls Exec\n",
		},
		{fixture: "fmt-errors", want: ""},
		{fixture: "file-close", want: ""},
		{
			fixture: "context",
			want: "web.Handler\n" +
				"  + context.Context: Greet calls Done; Greet calls Err\n",
		},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, report(t, test.fixture, changed.ByFunction))
		})
	}
}

// The mixed filter judges free functions by their body and methods by their
// package: fmt.Errorf and fmt.Sprintf drop out, while fakedb.Pool.Close and
// the context.Context interface stay.
func TestMixedFilter(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		fixture string
		want    string
	}{
		{
			fixture: "handler",
			want: "web.Handler\n" +
				"  + example.com/fakedb.Pool: Greet calls Exec\n",
		},
		{fixture: "fmt-errors", want: ""},
		{
			fixture: "file-close",
			want: "web.Handler\n" +
				"  + example.com/fakedb.Pool: Stop calls Close\n",
		},
		{
			fixture: "context",
			want: "web.Handler\n" +
				"  + context.Context: Greet calls Done; Greet calls Err\n",
		},
		{fixture: "pure", want: ""},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, report(t, test.fixture, changed.Mixed))
		})
	}
}

// Under the package filter the same Close is reported, since fakedb imports
// os.
func TestPackageFilterKeepsFileClose(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "web.Handler\n"+
		"  + example.com/fakedb.Pool: Stop calls Close\n",
		report(t, "file-close", changed.ByPackage))
}

// The Mermaid graph holds only the changed edges: added solid, removed
// dotted.
func TestMermaid(t *testing.T) {
	t.Parallel()
	dir := repo(t, "helper-refactor")
	base, err := changed.LoadRevision(dir, "HEAD~1", changed.ByPackage)
	require.NoError(t, err)
	head, err := changed.Load(dir, changed.ByPackage)
	require.NoError(t, err)
	assert.Equal(t, "graph LR\n"+
		"  n0[\"web.Handler\"]\n"+
		"  n1[\"example.com/fakedb.Pool\"]\n"+
		"  n0 -.->|-| n1\n"+
		"  n2[\"web.record\"]\n"+
		"  n2 -->|+| n1\n",
		changed.Mermaid(changed.Diff(base, head)))
}

// Reading the base revision leaves the repository as it was: no worktree
// added, no stash, nothing changed in the working tree.
func TestLoadRevisionLeavesRepositoryAlone(t *testing.T) {
	t.Parallel()
	dir := repo(t, "handler")
	worktrees := git(t, dir, "worktree", "list", "--porcelain")
	_, err := changed.LoadRevision(dir, "HEAD~1", changed.ByPackage)
	require.NoError(t, err)
	assert.Equal(t, worktrees, git(t, dir, "worktree", "list", "--porcelain"))
	assert.Equal(t, "", git(t, dir, "status", "--porcelain"))
	assert.Equal(t, "", git(t, dir, "stash", "list"))
}

// A revision that does not exist is an error, not an empty report.
func TestLoadRevisionUnknown(t *testing.T) {
	t.Parallel()
	_, err := changed.LoadRevision(repo(t, "handler"), "no-such-rev", changed.ByPackage)
	require.ErrorContains(t, err, "git archive no-such-rev")
}
