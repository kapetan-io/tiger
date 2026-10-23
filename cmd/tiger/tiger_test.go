package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/check"
	"github.com/kapetan-io/tiger/internal/command"
)

const layerModule = "layers.example/app"

// outcome is one check run's observable result.
type outcome struct {
	code   int
	stdout string
	stderr string
}

// run invokes the check command once; the leading word must be "check".
func run(t *testing.T, args ...string) outcome {
	t.Helper()
	require.Equal(t, "check", args[0])
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := check.Run(args[1:], command.Streams{Stdout: stdout, Stderr: stderr})
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// writeFiles writes new files under dir, creating directories as needed.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	require.NoError(t, os.CopyFS(dir, mapFiles(files)))
}

// writeModuleFiles writes files into a fresh directory.
func writeModuleFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	return dir
}

// mapFiles wraps file contents as an in-memory filesystem's entries.
func mapFiles(files map[string]string) fstest.MapFS {
	entries := fstest.MapFS{}
	for name, content := range files {
		entries[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return entries
}

// TestCheckOnThePatternsTreeIsSilent runs tiger check over the collection.
//
// Goal: every pattern, broken shapes included, passes tiger check with zero
// findings, the way the CI patterns job demands.
func TestCheckOnThePatternsTreeIsSilent(t *testing.T) {
	got := run(t, "check", "-C", "../..", "./patterns/...")
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestCheckPassesTigerItself runs tiger over its own module, which carries
// the capped worklists that replaced its uncapped ones.
//
// Goal: no finding, exit 0.
func TestCheckPassesTigerItself(t *testing.T) {
	got := run(t, "check", "-C", "../..", "./...")
	assert.Empty(t, got.stdout)
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stderr)
}

// TestLayersStandardLibraryFoundWithoutTheBuildGoroot covers the standard
// library contract for a tiger binary built with -trimpath whose own GOROOT is
// gone, as when the toolchain that built it was removed.
//
// Goal: fmt, strings and os are judged standard library through the go
// command's GOROOT, so a module whose every import is listed stays clean.
func TestLayersStandardLibraryFoundWithoutTheBuildGoroot(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "tiger")
	out, err := exec.Command("go", "build", "-trimpath",
		"-ldflags=-X=runtime.defaultGOROOT="+filepath.Join(t.TempDir(), "gone"),
		"-o", bin, "github.com/kapetan-io/tiger/cmd/tiger").CombinedOutput()
	assert.Empty(t, string(out))
	require.NoError(t, err)
	dir := writeModuleFiles(t, map[string]string{
		"go.mod": "module " + layerModule + "\n\ngo 1.26\n",
		"m.go":   "// Package app prints.\npackage app\n\nimport _ \"fmt\"\n",
		"store/store.go": "// Package store reads the environment.\npackage store\n\nimport (\n" +
			"\t_ \"os\"\n\t_ \"strings\"\n)\n",
		"tiger.layers.yaml": "\".\": []\n\"./store\":\n  - os\n",
	})

	check := exec.Command(bin, "check", "-C", dir, "./...")
	// A GOROOT in the environment would mask the binary's missing one.
	check.Env = slices.DeleteFunc(os.Environ(), func(variable string) bool {
		return strings.HasPrefix(variable, "GOROOT=")
	})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	check.Stdout, check.Stderr = stdout, stderr
	require.NoError(t, check.Run())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}
