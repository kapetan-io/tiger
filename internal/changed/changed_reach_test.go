package changed_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
)

// reachKey is the first line of the reach file for the toolchain, build tags
// and binary the tests run with.
func reachKey(t *testing.T) string {
	t.Helper()
	names := []string{
		"GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "GOEXPERIMENT", "GOFLAGS", "GOFIPS140",
		"GOAMD64", "GOARM", "GOARM64", "GO386", "GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64",
		"GOWASM",
	}
	out, err := exec.Command("go", append([]string{"env", "-json"}, names...)...).Output()
	require.NoError(t, err)
	var env map[string]string
	require.NoError(t, json.Unmarshal(out, &env))
	binary, err := os.Executable()
	require.NoError(t, err)
	data, err := os.ReadFile(binary)
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	parts := []string{"tiger-reachfile/2", "tiger=" + hex.EncodeToString(sum[:8])}
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%q", name, env[name]))
	}
	return strings.Join(parts, " ")
}

// reachEntry is where Go's build cache holds the reach file for a key.
func reachEntry(t *testing.T, key string) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOCACHE").Output()
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(strings.TrimSpace(string(out)), name[:2], name+"-d")
}

// firstLine is the first line of a file.
func firstLine(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	first, _, _ := strings.Cut(string(data), "\n")
	return first
}

// changedTogether runs tiger changed twice at once, as two processes sharing
// one build cache would.
func changedTogether(c checkout) [2]outcome {
	var runs [2]outcome
	var group sync.WaitGroup
	group.Go(func() { runs[0] = c.changed() })
	group.Go(func() { runs[1] = c.changed() })
	group.Wait()
	return runs
}

// whole reports whether the cache entry for a key names that key and ends
// with its trailer.
func whole(t *testing.T, key string) bool {
	t.Helper()
	data, err := os.ReadFile(reachEntry(t, key))
	require.NoError(t, err)
	return strings.HasPrefix(string(data), key+"\n") && strings.HasSuffix(string(data), "\nend\n")
}

// TestChangedKeepsTheReachFileInGoBuildCache covers where the reach file lives.
//
// Goal: tiger changed keeps the reach file as an entry in Go's build cache,
// never in the project; a missing, cut short or foreign entry is regenerated
// and the report does not change; a whole entry is read and its use recorded
// so Go's cache trim keeps it. The subtests share one cache entry, so none
// runs in parallel.
func TestChangedKeepsTheReachFileInGoBuildCache(t *testing.T) {
	key := reachKey(t)
	entry := reachEntry(t, key)
	require.NoError(t, os.RemoveAll(entry))
	c := fixture(t, "call-added")
	want := c.changed().stdout

	t.Run("stored", func(t *testing.T) {
		assert.Equal(t, key, firstLine(t, entry))
		assert.NoDirExists(t, filepath.Join(c.dir, ".tiger"))
	})

	t.Run("missing", func(t *testing.T) {
		require.NoError(t, os.Remove(entry))

		got := c.changed()
		assert.Equal(t, want, got.stdout)
		assert.True(t, whole(t, key))
	})

	t.Run("cutShort", func(t *testing.T) {
		require.NoError(t, os.WriteFile(entry, []byte(key+"\npackage-nobody-has\n"), 0o644))

		got := c.changed()
		assert.Equal(t, want, got.stdout)
		assert.True(t, whole(t, key))
	})

	t.Run("anotherKey", func(t *testing.T) {
		other := strings.Replace(key, " GOEXPERIMENT=\"", " GOEXPERIMENT=\"nobody", 1)
		require.NoError(t, os.WriteFile(entry, []byte(other+"\npackage-nobody-has\nend\n"), 0o644))

		got := c.changed()
		assert.Equal(t, want, got.stdout)
		assert.True(t, whole(t, key))
	})

	t.Run("matching", func(t *testing.T) {
		before, err := os.Stat(entry)
		require.NoError(t, err)

		got := c.changed()
		assert.Equal(t, command.ExitClean, got.code)
		after, err := os.Stat(entry)
		require.NoError(t, err)
		assert.Equal(t, before.ModTime(), after.ModTime())
	})

	t.Run("marksUse", func(t *testing.T) {
		old := time.Now().Add(-6 * 24 * time.Hour)
		require.NoError(t, os.Chtimes(entry, old, old))

		got := c.changed()
		assert.Equal(t, want, got.stdout)
		info, err := os.Stat(entry)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), info.ModTime(), time.Hour)
	})

	t.Run("concurrent", func(t *testing.T) {
		require.NoError(t, os.Remove(entry))

		runs := changedTogether(c)
		assert.Equal(t, want, runs[0].stdout)
		assert.Equal(t, want, runs[1].stdout)
		assert.True(t, whole(t, key))
		temps, err := filepath.Glob(strings.TrimSuffix(entry, "-d") + "-*-d")
		require.NoError(t, err)
		assert.Empty(t, temps)
	})

	t.Run("unwritable", func(t *testing.T) {
		require.NoError(t, os.Remove(entry))
		require.NoError(t, os.Mkdir(entry, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(entry, "keep"), nil, 0o644))
		t.Cleanup(func() { require.NoError(t, os.RemoveAll(entry)) })

		got := c.changed()
		assert.Equal(t, command.ExitClean, got.code)
		assert.Equal(t, want, got.stdout)
		assert.Contains(t, got.stderr, "cannot save the standard library reach file")
	})
}

// TestChangedCountsChangesOutsideTheDirectoryItRunsIn covers head naming.
//
// Goal: with -C pointing below the repository root, an uncommitted edit
// elsewhere in the repository still marks the head dirty, and the run itself
// leaves nothing in the project that would.
func TestChangedCountsChangesOutsideTheDirectoryItRunsIn(t *testing.T) {
	c := fixture(t, "call-added")
	clean := run(t, "changed", "-C", filepath.Join(c.dir, "web"), "--format", "json", "main")
	require.Equal(t, command.ExitClean, clean.code)
	assert.Contains(t, clean.stdout, `"dirty": false`)

	c.write("NOTES.md", "outside web\n")
	dirty := run(t, "changed", "-C", filepath.Join(c.dir, "web"), "--format", "json", "main")
	require.Equal(t, command.ExitClean, dirty.code)
	assert.Contains(t, dirty.stdout, `"dirty": true`)
}

// TestChangedKeysTheReachFileByBuildSettings covers the reach file's key.
//
// Goal: a build tag or cgo setting that changes which standard library files
// compile selects another cache entry, so a run never reads the answer made
// for other settings and leaves that entry as it was.
func TestChangedKeysTheReachFileByBuildSettings(t *testing.T) {
	c := fixture(t, "call-added")
	want := c.changed().stdout
	plain := reachEntry(t, reachKey(t))
	before, err := os.ReadFile(plain)
	require.NoError(t, err)

	for _, test := range []struct {
		name, env, value, inKey string
	}{
		{name: "netgoTag", env: "GOFLAGS", value: "-tags=netgo", inKey: `GOFLAGS="-tags=netgo"`},
		{name: "cgoOff", env: "CGO_ENABLED", value: "0", inKey: `CGO_ENABLED="0"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.env, test.value)
			key := reachKey(t)
			entry := reachEntry(t, key)
			require.NotEqual(t, plain, entry)

			got := c.changed()
			assert.Equal(t, want, got.stdout)
			assert.Contains(t, key, test.inKey)
			assert.True(t, whole(t, key))
			after, err := os.ReadFile(plain)
			require.NoError(t, err)
			assert.Equal(t, before, after)
		})
	}
}
