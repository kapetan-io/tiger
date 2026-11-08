package changed_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
)

// scratchCache points Go at an empty build cache for the test, so nothing it
// does touches the real one.
func scratchCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GOCACHE", dir)
	return dir
}

// TestChangedReadsTheReachFileFromARestoredCache covers a CI job that restores
// a saved Go build cache.
//
// Goal: a run whose build cache holds the reach file from an earlier run, at
// another path and with the same tiger binary, reads it instead of making a
// new one.
func TestChangedReadsTheReachFileFromARestoredCache(t *testing.T) {
	c := fixture(t, "call-added")
	want := c.changed().stdout
	entry := reachEntry(t, reachKey(t))
	saved, err := os.ReadFile(entry)
	require.NoError(t, err)

	scratchCache(t)
	stamp := time.Now().Add(-30 * time.Minute)
	path := reachEntry(t, reachKey(t))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, saved, 0o644))
	require.NoError(t, os.Chtimes(path, stamp, stamp))

	got := c.changed()

	assert.Equal(t, command.ExitClean, got.code)
	assert.Equal(t, want, got.stdout)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.WithinDuration(t, stamp, info.ModTime(), time.Second)
}

// TestChangedWorksWithACacheProgram covers GOCACHEPROG.
//
// Goal: with a cache program set, tiger changed prints the report it prints
// without one.
func TestChangedWorksWithACacheProgram(t *testing.T) {
	c := fixture(t, "call-added")
	want := c.changed().stdout

	program := filepath.Join(t.TempDir(), "cacheprog")
	build := exec.Command("go", "build", "-o", program, "./testdata/cacheprog")
	_, err := build.CombinedOutput()
	require.NoError(t, err)
	scratchCache(t)
	t.Setenv("GOCACHEPROG", program+" "+t.TempDir())

	got := c.changed()

	assert.Equal(t, command.ExitClean, got.code)
	assert.Equal(t, want, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestChangedReachFileSurvivesGoTrimOnlyWhileUsed covers Go's cache trim.
//
// Goal: Go's trim deletes a reach file unused for more than five days, and
// keeps one that a tiger changed run refreshed.
func TestChangedReachFileSurvivesGoTrimOnlyWhileUsed(t *testing.T) {
	c := fixture(t, "call-added")
	cache := scratchCache(t)
	require.Equal(t, command.ExitClean, c.changed().code)
	entry := reachEntry(t, reachKey(t))
	require.FileExists(t, entry)

	module := t.TempDir()
	goMod := []byte("module example.com/trim\n\ngo 1.25.0\n")
	require.NoError(t, os.WriteFile(filepath.Join(module, "go.mod"), goMod, 0o644))
	source := []byte("package trim\n")
	require.NoError(t, os.WriteFile(filepath.Join(module, "trim.go"), source, 0o644))
	goBuild := func() {
		t.Helper()
		// Go trims at most once a day, so a stale trim.txt lets this build do it.
		require.NoError(t, os.WriteFile(filepath.Join(cache, "trim.txt"), []byte("1"), 0o644))
		build := exec.Command("go", "build", "./...")
		build.Dir = module
		_, err := build.CombinedOutput()
		require.NoError(t, err)
	}

	old := time.Now().Add(-6 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(entry, old, old))
	require.Equal(t, command.ExitClean, c.changed().code)
	goBuild()
	assert.FileExists(t, entry)

	require.NoError(t, os.Chtimes(entry, old, old))
	goBuild()
	assert.NoFileExists(t, entry)
}
