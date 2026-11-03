package changed_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
)

// reachKey is the first line a reach file carries for the toolchain the tests
// run with.
func reachKey(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOVERSION", "GOOS", "GOARCH").Output()
	require.NoError(t, err)
	return "tiger-stdreach/1 " + strings.Join(strings.Fields(string(out)), " ")
}

// reachPath is where a project keeps the reach file for the toolchain in use,
// relative to the module root.
func reachPath(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOOS", "GOARCH").Output()
	require.NoError(t, err)
	system := strings.Fields(string(out))
	return filepath.Join(".tiger", "stdreach-"+system[0]+"-"+system[1]+".txt")
}

// firstLine is the first line of a file.
func firstLine(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	first, _, _ := strings.Cut(string(data), "\n")
	return first
}

// TestChangedKeepsTheStandardLibraryReachInTheProject covers the reach file.
//
// Goal: with no reach file, or one made by another Go version, tiger changed
// writes the file for the toolchain in use inside the project and prints the
// same report; a file that matches is read and left untouched.
func TestChangedKeepsTheStandardLibraryReachInTheProject(t *testing.T) {
	file := reachPath(t)

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		c := fixture(t, "call-added")
		want := c.changed().stdout
		require.NoError(t, os.RemoveAll(filepath.Join(c.dir, ".tiger")))

		got := c.changed()
		assert.Equal(t, want, got.stdout)
		assert.Equal(t, reachKey(t), firstLine(t, filepath.Join(c.dir, file)))
	})

	t.Run("anotherVersion", func(t *testing.T) {
		t.Parallel()
		c := fixture(t, "call-added")
		want := c.changed().stdout
		key := strings.Fields(reachKey(t))
		key[1] = "go1.0"
		c.write(repoPath(filepath.ToSlash(file)), strings.Join(key, " ")+"\npackage-nobody-has\n")

		got := c.changed()
		assert.Equal(t, want, got.stdout)
		assert.Equal(t, reachKey(t), firstLine(t, filepath.Join(c.dir, file)))
	})

	t.Run("moduleRoot", func(t *testing.T) {
		t.Parallel()
		c := fixture(t, "call-added")
		require.NoError(t, os.RemoveAll(filepath.Join(c.dir, ".tiger")))

		got := run(t, "changed", "-C", filepath.Join(c.dir, "web"), "main")
		assert.Equal(t, command.ExitClean, got.code)
		assert.Equal(t, reachKey(t), firstLine(t, filepath.Join(c.dir, file)))
		assert.NoFileExists(t, filepath.Join(c.dir, "web", ".tiger"))
	})

	t.Run("unwritable", func(t *testing.T) {
		t.Parallel()
		c := fixture(t, "call-added")
		want := c.changed().stdout
		dir := filepath.Join(c.dir, ".tiger")
		require.NoError(t, os.RemoveAll(dir))
		require.NoError(t, os.Mkdir(dir, 0o500))
		t.Cleanup(func() { assert.NoError(t, os.Chmod(dir, 0o755)) })

		got := c.changed()
		assert.Equal(t, command.ExitClean, got.code)
		assert.Equal(t, want, got.stdout)
		assert.Contains(t, got.stderr, "cannot save the standard library reach file")
	})

	t.Run("matching", func(t *testing.T) {
		t.Parallel()
		c := fixture(t, "call-added")
		path := filepath.Join(c.dir, file)
		before, err := os.Stat(path)
		require.NoError(t, err)

		got := c.changed()
		assert.Equal(t, command.ExitClean, got.code)
		after, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, before.ModTime(), after.ModTime())
	})
}

// TestChangedIgnoresItsOwnReachFileWhenCountingChanges covers head naming.
//
// Goal: a reach file the project neither commits nor ignores does not make the
// head dirty, so the first and the second run print the same report.
func TestChangedIgnoresItsOwnReachFileWhenCountingChanges(t *testing.T) {
	c := fixture(t, "call-added")
	// An empty exclude file is a project that has not decided about .tiger.
	c.write(".git/info/exclude", "")
	first := c.report()
	second := c.report()
	assert.Equal(t, false, first.Head["dirty"])
	assert.Equal(t, first.Head, second.Head)
}

// TestChangedReadsAReachFileTheProjectCommitted covers committing the file.
//
// Goal: a project that commits its reach file gets the same report on every
// run, with a clean head and the file left as committed.
func TestChangedReadsAReachFileTheProjectCommitted(t *testing.T) {
	c := fixture(t, "call-added")
	c.write(".git/info/exclude", "")
	c.git("add", "-f", ".tiger")
	c.commit("commit the reach file")
	before, err := os.Stat(filepath.Join(c.dir, reachPath(t)))
	require.NoError(t, err)

	first := c.report()
	second := c.report()
	assert.Equal(t, false, first.Head["dirty"])
	assert.Equal(t, first.Head, second.Head)
	after, err := os.Stat(filepath.Join(c.dir, reachPath(t)))
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())
}

// TestChangedRegeneratesAReachFileThatWasCutShort covers a damaged file.
//
// Goal: a reach file with the right first line but missing its end is read as
// stale and rewritten whole, and the report matches the one from a whole file.
func TestChangedRegeneratesAReachFileThatWasCutShort(t *testing.T) {
	c := fixture(t, "call-added")
	want := c.changed().stdout
	c.write(repoPath(".tiger/"+reachFile.name), firstLine(t, filepath.Join(c.dir, ".tiger", reachFile.name))+"\npackage-nobody-has\n")

	got := c.changed()
	assert.Equal(t, want, got.stdout)
	data, err := os.ReadFile(filepath.Join(c.dir, ".tiger", reachFile.name))
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(data), "\nend\n"))
}
