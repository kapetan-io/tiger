package cli_test

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

var update = flag.Bool("update", false, "rewrite the tiger changed golden files")

const shopModule = `module example.com/shop

go 1.26

require example.com/fakedb v0.0.0

replace example.com/fakedb => ./fakedb
`

// checkout is a real git repository in a temporary directory. Authors and
// dates are pinned so commit hashes, and the titles that print them, are the
// same on every machine.
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
	return c
}

func (c checkout) git(args ...string) string {
	c.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = c.dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
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

func (c checkout) read(name repoPath) string {
	c.t.Helper()
	data, err := os.ReadFile(filepath.Join(c.dir, filepath.FromSlash(string(name))))
	require.NoError(c.t, err)
	return string(data)
}

func (c checkout) commit(message string) {
	c.t.Helper()
	c.git("add", "-A")
	c.git("commit", "-q", "-m", message)
}

// short is a revision's seven-character hash, as the report prints it.
func (c checkout) short(rev string) string {
	c.t.Helper()
	return c.git("rev-parse", "--short=7", rev)
}

// copyFixture copies a directory of testdata/changed into the checkout root.
func (c checkout) copyFixture(name string, dest repoPath) {
	c.t.Helper()
	source := os.DirFS(filepath.Join("testdata", "changed", filepath.FromSlash(name)))
	require.NoError(c.t, os.CopyFS(filepath.Join(c.dir, string(dest)), source))
}

// fixture commits testdata/changed/<name>/base on main, then the head
// directory on a branch called feature, and leaves feature checked out. Files
// in head-ignored are copied last, so a .gitignore in head keeps them out of git.
func fixture(t *testing.T, name string) checkout {
	t.Helper()
	c := newCheckout(t)
	c.write("go.mod", shopModule)
	c.copyFixture("fakedb", "fakedb")
	c.copyFixture(name+"/base", ".")
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")

	entries, err := os.ReadDir(filepath.Join("testdata", "changed", name, "base"))
	require.NoError(t, err)
	tracked := []string{"rm", "-rfq", "--"}
	for _, entry := range entries {
		tracked = append(tracked, entry.Name())
	}
	c.git(tracked...)
	c.copyFixture(name+"/head", ".")
	c.commit("head")

	ignored := filepath.Join("testdata", "changed", name, "head-ignored")
	if _, err := os.Stat(ignored); err == nil {
		c.copyFixture(name+"/head-ignored", ".")
	}
	return c
}

// changed runs tiger changed against main from the checkout.
func (c checkout) changed(args ...string) outcome {
	c.t.Helper()
	return run(c.t, append([]string{"changed", "-C", c.dir}, append(args, "main")...)...)
}

// golden compares the report with testdata/changed/<name>/text.golden, or
// rewrites the file when the test binary runs with -update.
func golden(t *testing.T, name string, got outcome) {
	t.Helper()
	path := filepath.Join("testdata", "changed", name, "text.golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got.stdout), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got.stdout)
}

// TestChangedGoldens covers the report's exact bytes for each shape of change.
//
// Goal: the text report of every case equals its golden file, exits 0, and
// writes nothing to stderr.
func TestChangedGoldens(t *testing.T) {
	for _, test := range []struct {
		name string
	}{
		{name: "call-added"},
		{name: "call-removed"},
		{name: "sibling-owner"},
		{name: "unchanged-caller"},
		{name: "line-shift"},
		{name: "empty"},
		{name: "new-package"},
		{name: "removed-package"},
		{name: "rename"},
		{name: "rename-changed-calls"},
		{name: "module-helper"},
		{name: "closure"},
		{name: "interface-call"},
		{name: "reach-not-propagated"},
		{name: "method-refactor"},
		{name: "helper-refactor"},
		{name: "writer-silent"},
		{name: "channel-send"},
		{name: "channel-receive"},
		{name: "recursive-callee"},
		{name: "mixed-filter-function"},
		{name: "mixed-filter-method"},
		{name: "outside-pure-helpers"},
		{name: "context-shown"},
		{name: "generated-head-only"},
		{name: "partial-load"},
		{name: "sibling-owner-removed"},
		{name: "owner-replaced"},
		{name: "generated-base-only"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := fixture(t, test.name).changed()
			assert.Equal(t, cli.ExitClean, got.code)
			assert.Empty(t, got.stderr)
			golden(t, test.name, got)
		})
	}
}

const queueSource = `package store

import "example.com/fakedb"

type Queue struct{ pool *fakedb.Pool }

func (q *Queue) Retry() error { return q.pool.Exec("retry") }
`

// TestChangedIgnoresCommitsLandedOnTheBaseAfterTheSplit covers the merge base.
//
// Goal: a commit that lands on main after the branch split never appears.
func TestChangedIgnoresCommitsLandedOnTheBaseAfterTheSplit(t *testing.T) {
	c := fixture(t, "call-added")
	c.git("checkout", "-q", "main")
	c.write("store/store.go", queueSource)
	c.commit("main moves on")
	c.git("checkout", "-q", "feature")

	got := c.changed()
	require.Equal(t, cli.ExitClean, got.code)
	assert.NotContains(t, got.stdout, "store.Queue")
	assert.Contains(t, got.stdout, "web.Handler.Greet\n+   example.com/fakedb.Pool.Exec\n")
	assert.Contains(t, got.stdout, "tiger changed: main (merge base "+c.short("main~1")+") -> ")
}

// TestChangedNamesHeadByCommit covers head naming.
//
// Goal: a clean head prints its short hash alone with the same bytes on a
// branch and on a detached HEAD, and a dirty tree adds its uncommitted edits
// to the report and says so.
func TestChangedNamesHeadByCommit(t *testing.T) {
	c := fixture(t, "call-added")
	onBranch := c.changed()
	require.Equal(t, cli.ExitClean, onBranch.code)
	head := c.short("HEAD")
	assert.Contains(t, onBranch.stdout, " -> "+head+"\n")

	c.git("checkout", "-q", "--detach")
	assert.Equal(t, onBranch.stdout, c.changed().stdout)

	edited := strings.Replace(c.read("web/web.go"),
		`return "hello " + name`, "_, _ = h.pool.Query(name)\n\treturn \"hello \" + name", 1)
	c.write("web/web.go", edited)
	dirty := c.changed()
	require.Equal(t, cli.ExitClean, dirty.code)
	assert.Contains(t, dirty.stdout, " -> "+head+" + uncommitted changes\n")
	assert.Contains(t, dirty.stdout, "+   example.com/fakedb.Pool.Query\n")
	assert.Contains(t, dirty.stdout, "Calls added: 2 ·")
}

// TestChangedPrintsIdenticalBytes covers determinism.
//
// Goal: the same input prints the same bytes on a second run and with one
// scheduler thread.
func TestChangedPrintsIdenticalBytes(t *testing.T) {
	c := fixture(t, "new-package")
	first := c.changed()
	require.Equal(t, cli.ExitClean, first.code)
	assert.Equal(t, first.stdout, c.changed().stdout)

	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	assert.Equal(t, first.stdout, c.changed().stdout)
}

// TestChangedRefusesWhatItCannotCompare covers the exit-2 contract.
//
// Goal: every run that cannot produce a report prints one line on stderr,
// nothing on stdout, and exits 2.
func TestChangedRefusesWhatItCannotCompare(t *testing.T) {
	c := fixture(t, "call-added")
	c.git("checkout", "-q", "--orphan", "unrelated")
	c.git("rm", "-rfq", ".")
	c.write("README.md", "unrelated\n")
	c.commit("unrelated history")
	c.git("checkout", "-q", "feature")

	outside := t.TempDir()
	for _, test := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknownRevision",
			args:    []string{"changed", "-C", c.dir, "nope"},
			wantErr: "cannot find revision nope",
		},
		{
			name:    "flagAfterBase",
			args:    []string{"changed", "-C", c.dir, "main", "--format", "text"},
			wantErr: "comes after the base revision",
		},
		{
			name:    "dashedBase",
			args:    []string{"changed", "-C", c.dir, "--", "--output=x"},
			wantErr: "starts with a dash",
		},
		{
			name:    "noMergeBase",
			args:    []string{"changed", "-C", c.dir, "unrelated"},
			wantErr: "no common ancestor of unrelated and HEAD",
		},
		{
			name:    "noBase",
			args:    []string{"changed", "-C", c.dir},
			wantErr: "expected a base revision",
		},
		{
			name:    "unknownFormat",
			args:    []string{"changed", "-C", c.dir, "--format", "markdown", "main"},
			wantErr: `unknown format "markdown"`,
		},
		{
			name:    "unknownFlag",
			args:    []string{"changed", "-C", c.dir, "--link", "main"},
			wantErr: "flag provided but not defined: -link",
		},
		{
			name:    "notARepository",
			args:    []string{"changed", "-C", outside, "main"},
			wantErr: "run it inside a git checkout",
		},
		{
			name:    "missingDirectory",
			args:    []string{"changed", "-C", filepath.Join(outside, "absent"), "main"},
			wantErr: "cannot read directory",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, test.args...)
			assert.Equal(t, cli.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Equal(t, 1, strings.Count(got.stderr, "\n"))
			assert.Contains(t, got.stderr, test.wantErr)
		})
	}
}

// gitState is everything a run must leave as it found it.
func (c checkout) gitState() []string {
	return []string{
		c.git("status", "--porcelain"),
		c.git("stash", "list"),
		c.git("worktree", "list"),
		c.read(".git/index"),
	}
}

// TestChangedLeavesTheRepositoryAlone covers invariant 6.
//
// Goal: a run on a dirty tree changes neither the status, the stash, the
// worktree list nor the index file.
func TestChangedLeavesTheRepositoryAlone(t *testing.T) {
	c := fixture(t, "call-added")
	c.write("web/untracked.go", "package web\n")
	c.git("stash", "push", "-q", "--include-untracked", "-m", "keep")
	c.write("web/untracked.go", "package web\n")
	c.write("README.md", "edited\n")

	before := c.gitState()
	got := c.changed()
	require.Equal(t, cli.ExitClean, got.code)
	assert.Equal(t, before, c.gitState())
}

const openSource = `package web

import "os"

func Open() error {
	_, err := os.Open(".")
	return err
}
`

// TestChangedRunDirectoryMissingAtBase covers the new-module case.
//
// Goal: when the run directory did not exist at the merge base every package
// prints as new and the command exits 0.
func TestChangedRunDirectoryMissingAtBase(t *testing.T) {
	c := newCheckout(t)
	c.write("README.md", "docs only\n")
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.write("svc/go.mod", "module example.com/svc\n\ngo 1.26\n")
	c.write("svc/web/web.go", openSource)
	c.commit("add service")

	got := run(t, "changed", "-C", filepath.Join(c.dir, "svc"), "main")
	require.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout, "Calls added: 0 · Calls removed: 0 · New packages: 1\n")
	assert.Contains(t, got.stdout, "New package web: 1 types and functions, 1 calls\n")
	assert.Contains(t, got.stdout, "    web.Open\n    +   os.Open\n")
}

const (
	appBase = `package app

type Server struct{}

func (s *Server) Start() {}
`
	appHead = `package app

import (
	"example.com/mono/app/internal/store"
	"example.com/mono/libs/ids"
)

type Server struct{}

func (s *Server) Start() {
	_ = ids.New()
	_ = store.Save()
}
`
	storeSource = `package store

import "os"

func Save() error {
	_, err := os.Create("x")
	return err
}
`
)

// TestChangedNamesPackagesRelativeToTheRunDirectory covers section 3 naming.
//
// Goal: packages under the run directory print relative to it, the package at
// the run directory by its name, and module packages above it relative to the
// module root.
func TestChangedNamesPackagesRelativeToTheRunDirectory(t *testing.T) {
	c := newCheckout(t)
	c.write("go.mod", "module example.com/mono\n\ngo 1.26\n")
	c.write("libs/ids/ids.go", "package ids\n\nfunc New() string { return \"id\" }\n")
	c.write("app/internal/store/store.go", storeSource)
	c.write("app/app.go", appBase)
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.write("app/app.go", appHead)
	c.commit("head")

	got := run(t, "changed", "-C", filepath.Join(c.dir, "app"), "main")
	require.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout,
		"\napp.Server.Start\n+   internal/store.Save\n+   libs/ids.New\n\n")
}

// TestChangedReportsAnEmptyDiffWithItsFooter covers the empty report.
//
// Goal: a branch that changes no Go file prints the empty sentence and both
// footer lines and exits 0.
func TestChangedReportsAnEmptyDiffWithItsFooter(t *testing.T) {
	c := fixture(t, "empty")
	c.git("reset", "-q", "--hard", "main")
	c.write("README.md", "docs\n")
	c.commit("docs")

	got := c.changed()
	require.Equal(t, cli.ExitClean, got.code)
	lines := strings.Split(strings.TrimSuffix(got.stdout, "\n"), "\n")
	assert.Equal(t, "Calls added: 0 · Calls removed: 0", lines[3])
	assert.Equal(t, "No function started or stopped calling another package, "+
		"or a function in its own package that calls one.", lines[5])
	assert.True(t, strings.HasPrefix(lines[len(lines)-2],
		"Compared 1 types and functions in 1 packages; skipped all test files"))
	assert.True(t, strings.HasPrefix(lines[len(lines)-1], "Tiger can't see:"))
}

// TestChangedCountsEqualTheVisibleLines covers invariant 3 and the single
// category of each difference.
//
// Goal: the numbers on the counts line equal the unindented + and - lines of
// the main list, so a call folded under a new package or a removed package is
// never also counted in the main list.
func TestChangedCountsEqualTheVisibleLines(t *testing.T) {
	for _, test := range []struct {
		name    string
		added   int
		removed int
	}{
		{name: "call-added", added: 1},
		{name: "call-removed", removed: 1},
		{name: "sibling-owner", added: 2},
		{name: "sibling-owner-removed", removed: 2},
		{name: "unchanged-caller", added: 1, removed: 1},
		{name: "new-package", added: 1},
		{name: "removed-package", removed: 1},
		{name: "rename"},
		{name: "rename-changed-calls", added: 2, removed: 1},
		{name: "owner-replaced", added: 1, removed: 1},
		{name: "closure", added: 2},
		{name: "partial-load", added: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := fixture(t, test.name).changed()
			require.Equal(t, cli.ExitClean, got.code)
			var added, removed int
			for _, line := range strings.Split(got.stdout, "\n") {
				switch {
				case strings.HasPrefix(line, "+   "):
					added++
				case strings.HasPrefix(line, "-   "):
					removed++
				}
			}
			assert.Equal(t, test.added, added)
			assert.Equal(t, test.removed, removed)
			assert.Contains(t, got.stdout,
				fmt.Sprintf("\nCalls added: %d · Calls removed: %d", test.added, test.removed))
		})
	}
}

// TestChangedNamesTheBaseRevision covers the base label.
//
// Goal: the title prints the ref and its short hash for a branch name, the
// short hash alone for a hash prefix of any length, and the merge base when
// it differs from the tip of the ref, whether the ref is a name or a hash.
func TestChangedNamesTheBaseRevision(t *testing.T) {
	c := fixture(t, "call-added")
	head := c.short("HEAD")
	title := func(base string) string {
		got := run(t, "changed", "-C", c.dir, base)
		require.Equal(t, cli.ExitClean, got.code)
		first, _, _ := strings.Cut(got.stdout, "\n")
		return first
	}

	tip := c.git("rev-parse", "main")
	split := c.short("main")
	assert.Equal(t, "tiger changed: main ("+split+") -> "+head, title("main"))
	assert.Equal(t, "tiger changed: "+split+" -> "+head, title(split))
	assert.Equal(t, "tiger changed: "+split+" -> "+head, title(tip[:10]))
	assert.Equal(t, "tiger changed: "+split+" -> "+head, title(tip))
	assert.Equal(t, "tiger changed: HEAD~1 ("+split+") -> "+head, title("HEAD~1"))

	c.git("checkout", "-q", "main")
	c.write("store/store.go", queueSource)
	c.commit("main moves on")
	c.git("checkout", "-q", "feature")
	moved := c.short("main")
	assert.Equal(t, "tiger changed: main (merge base "+split+") -> "+head, title("main"))
	assert.Equal(t, "tiger changed: "+moved+" (merge base "+split+") -> "+head, title(moved))
	assert.Equal(t, "tiger changed: main~1 ("+split+") -> "+head, title("main~1"))
}

const extraSource = `package web

import "example.com/fakedb"

func Extra() error {
	return fakedb.Ping(1)
}
`

// TestChangedCountsAnUntrackedFileAsUncommitted covers the dirty head.
//
// Goal: a Go file git has never seen marks the head as having uncommitted
// changes, and its calls are reported.
func TestChangedCountsAnUntrackedFileAsUncommitted(t *testing.T) {
	c := fixture(t, "call-added")
	c.write("web/extra.go", extraSource)

	got := c.changed()
	require.Equal(t, cli.ExitClean, got.code)
	assert.Contains(t, got.stdout, " -> "+c.short("HEAD")+" + uncommitted changes\n")
	assert.Contains(t, got.stdout, "web.Extra\n+   example.com/fakedb.Ping\n")
	assert.Contains(t, got.stdout, "Calls added: 2 · Calls removed: 0\n")
}

// TestChangedPrintsIdenticalBytesForSeveralChanges covers determinism.
//
// Goal: reports with several functions, callees, removals and a rename print
// the same bytes on every run, whatever order the maps inside the tool visit.
func TestChangedPrintsIdenticalBytesForSeveralChanges(t *testing.T) {
	for _, name := range []string{"sibling-owner", "closure", "rename-changed-calls"} {
		c := fixture(t, name)
		first := c.changed()
		require.Equal(t, cli.ExitClean, first.code)
		for range 4 {
			assert.Equal(t, first.stdout, c.changed().stdout)
		}
	}
}

const spinSource = `package fakedb

// Spin and Twirl call each other and never reach os, so a walk that finds
// nothing has to stop on the cycle.
func Spin(n int) int {
	if n <= 0 {
		return 0
	}
	return Twirl(n - 1)
}

func Twirl(n int) int {
	return Spin(n)
}
`

// TestChangedStopsOnARecursiveOutsideFunction covers the syscall-reach walk.
//
// Goal: a free function of an outside package whose body recurses without ever
// reaching the operating system is left out, the walk ends, and a sibling that
// does reach it still prints.
func TestChangedStopsOnARecursiveOutsideFunction(t *testing.T) {
	c := newCheckout(t)
	c.write("go.mod", shopModule)
	c.copyFixture("fakedb", "fakedb")
	c.write("fakedb/spin.go", spinSource)
	c.copyFixture("recursive-pure/base", ".")
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.git("rm", "-rfq", "--", "web")
	c.copyFixture("recursive-pure/head", ".")
	c.commit("head")

	got := c.changed()
	require.Equal(t, cli.ExitClean, got.code)
	assert.Contains(t, got.stdout, "Calls added: 1 · Calls removed: 0\n")
	assert.Contains(t, got.stdout, "web.Handler.Greet\n+   example.com/fakedb.Ping\n\n")
	assert.NotContains(t, got.stdout, "Spin")
}
