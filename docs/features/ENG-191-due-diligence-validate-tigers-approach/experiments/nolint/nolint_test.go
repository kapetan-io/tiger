package nolint_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture copies testdata/common and then testdata/<name> into a fresh
// module, so a fixture's own .golangci.yml replaces the common one.
func fixture(t *testing.T, name string) string {
	// EvalSymlinks because golangci-lint mishandles a symlinked working
	// directory (see experiments/newfromrev).
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	copyTree(t, "testdata/common", dir)
	copyTree(t, filepath.Join("testdata", name), dir)
	return dir
}

func copyTree(t *testing.T, from, to string) {
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), data, 0o644)
	})
	require.NoError(t, err)
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

// TS-E02 reports an error dropped with no comment.
func TestDroppedErrorFires(t *testing.T) {
	out, code := run(t, fixture(t, "dropped"), binary(t, "TIGER"), "check", "./...")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "TS-E02")
}

// Under tiger check a bare //nolint silences nothing by itself, but it counts
// as the comment TS-E02 asks for, so the same dropped error passes.
func TestBareNolintSatisfiesDroppedError(t *testing.T) {
	out, code := run(t, fixture(t, "droppednolint"), binary(t, "TIGER"), "check", "./...")
	assert.Equal(t, 0, code)
	assert.NotContains(t, out, "TS-E02")
}

// tiger check does not read //nolint:tiger, so the finding on that line stays.
func TestTigerCheckIgnoresLineNolint(t *testing.T) {
	out, code := run(t, fixture(t, "line"), binary(t, "TIGER"), "check", "./...")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "probe.go:11:5: TS-S09")
}

// Under the plugin, //nolint:tiger on the line drops the finding and the run
// passes.
func TestPluginLineNolintSilencesFinding(t *testing.T) {
	out, code := run(t, fixture(t, "line"), binary(t, "TIGER_GCL"), "run", "./...")
	assert.Equal(t, 0, code)
	assert.NotContains(t, out, "TS-S09")
}

// One //nolint:tiger above the package clause drops every tiger finding in the
// file under the plugin, including one at the package line, which is where an
// opt-in "no //nolint" finding would be reported. tiger check reports both.
func TestFileNolintSilencesWholeFile(t *testing.T) {
	dir := fixture(t, "filewide")
	out, code := run(t, dir, binary(t, "TIGER"), "check", "./...")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "probe.go:6:1: TS-P02")
	assert.Contains(t, out, "TS-S09")
	out, code = run(t, dir, binary(t, "TIGER_GCL"), "run", "./...")
	assert.Equal(t, 0, code)
	assert.NotContains(t, out, "TS-P02")
	assert.NotContains(t, out, "TS-S09")
}

// An exclusions rule in .golangci.yml drops the finding under the plugin with
// no comment in the code at all. tiger check does not read .golangci.yml.
func TestExclusionsSilenceFinding(t *testing.T) {
	dir := fixture(t, "excluded")
	out, code := run(t, dir, binary(t, "TIGER"), "check", "./...")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "TS-S09")
	out, code = run(t, dir, binary(t, "TIGER_GCL"), "run", "./...")
	assert.Equal(t, 0, code)
	assert.NotContains(t, out, "TS-S09")
}
