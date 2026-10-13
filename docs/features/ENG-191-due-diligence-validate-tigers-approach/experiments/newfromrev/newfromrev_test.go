package newfromrev_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The plugin prints "old.go:12:5: nogoto: TS-S09" and tiger check prints
// "old.go:12:5: TS-S09", so the tests match the position only.
const (
	oldFinding = "old.go:12:5:"
	newFinding = "new.go:12:5:"
)

// repo builds a git repository with two commits: old.go on the main line,
// then new.go on the branch. Both hold the same TS-S09 violation.
func repo(t *testing.T) string {
	// t.TempDir is under /var on macOS, a symlink to /private/var, and
	// TestNewFromRevThroughSymlinkHidesEverything shows what that does.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	copyFile(t, "testdata/repo/.golangci.yml", filepath.Join(dir, ".golangci.yml"))
	copyFile(t, "testdata/repo/go.mod.txt", filepath.Join(dir, "go.mod"))
	copyFile(t, "testdata/repo/old.go", filepath.Join(dir, "old.go"))
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "main")
	copyFile(t, "testdata/repo/new.go.txt", filepath.Join(dir, "new.go"))
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "branch")
	return dir
}

func copyFile(t *testing.T, from, to string) {
	data, err := os.ReadFile(from)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(to, data, 0o644))
}

func git(t *testing.T, dir string, args ...string) {
	cmd := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// run returns the combined output and exit code of a command run in dir.
func run(t *testing.T, dir, binary string, args ...string) (string, int) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok {
		return string(out), exit.ExitCode()
	}
	require.NoError(t, err, string(out))
	return string(out), 0
}

func binary(t *testing.T, name string) string {
	path := os.Getenv(name)
	require.NotEmpty(t, path)
	return path
}

// Without --new-from-rev, the tiger plugin reports both violations and the
// run fails.
func TestPluginReportsBoth(t *testing.T) {
	out, code := run(t, repo(t), binary(t, "TIGER_GCL"), "run", "./...")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, oldFinding)
	assert.Contains(t, out, newFinding)
}

// With the flag tiger's own template recommends, golangci-lint drops the
// finding on the line the branch didn't touch, and prints nothing about it.
func TestNewFromRevHidesOldFinding(t *testing.T) {
	out, code := run(t, repo(t), binary(t, "TIGER_GCL"), "run", "--new-from-rev=HEAD~1", "./...")
	assert.Equal(t, 1, code)
	assert.NotContains(t, out, oldFinding)
	assert.Contains(t, out, newFinding)
}

// Measured from the branch's own head, every finding is old, so a repository
// with two blocking findings passes.
func TestNewFromRevPassesEverything(t *testing.T) {
	out, code := run(t, repo(t), binary(t, "TIGER_GCL"), "run", "--new-from-rev=HEAD", "./...")
	assert.Equal(t, 0, code)
	assert.NotContains(t, out, "TS-S09")
}

// Run from a path that reaches the repository through a symlink,
// --new-from-rev drops every finding, including the one the branch added.
func TestNewFromRevThroughSymlinkHidesEverything(t *testing.T) {
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(repo(t), link))
	out, code := run(t, link, binary(t, "TIGER_GCL"), "run", "--new-from-rev=HEAD~1", "./...")
	assert.Equal(t, 0, code)
	assert.NotContains(t, out, "TS-S09")
}

// tiger check has no git filter, so it reports both on the same repository.
func TestTigerCheckReportsBoth(t *testing.T) {
	out, code := run(t, repo(t), binary(t, "TIGER"), "check", "./...")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, oldFinding)
	assert.Contains(t, out, newFinding)
}
