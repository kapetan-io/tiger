package changed_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.uber.org/goleak"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/changed"
	"github.com/kapetan-io/tiger/internal/command"
)

// One run before the tests leaves the reach file in Go's build cache, so the
// tests that run in parallel read it instead of each type-checking the
// standard library at once.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tiger-reach-seed")
	assert.NoError(err, "temporary directory")
	module := []byte("module example.com/seed\n\ngo 1.26\n")
	assert.NoError(os.WriteFile(filepath.Join(dir, "go.mod"), module, 0o644), "go.mod")
	source := []byte("package seed\n")
	assert.NoError(os.WriteFile(filepath.Join(dir, "seed.go"), source, 0o644), "seed.go")
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		assert.NoError(err, string(out))
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("-c", "user.name=seed", "-c", "user.email=seed@example.com", "commit", "-q", "-m", "seed")

	discard := command.Streams{Stdout: io.Discard, Stderr: io.Discard}
	code := changed.Run([]string{"-C", dir, "main"}, discard)
	assert.Equal(code, command.ExitClean, "generating the reach file")
	assert.NoError(os.RemoveAll(dir), "remove "+dir)

	goleak.VerifyTestMain(m)
}
