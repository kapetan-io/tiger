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

// reachFile is a reach file generated once for the test binary, named as it
// is inside a project. A project that keeps the file in its tree reads it
// instead of type-checking the standard library again; every checkout gets a
// copy, ignored by git the way a project that does not commit it would.
var reachFile struct {
	name string
	data []byte
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tiger-reach-seed")
	assert.NoError(err, "temporary directory")
	defer func() { assert.NoError(os.RemoveAll(dir), "remove "+dir) }()
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
	files, err := filepath.Glob(filepath.Join(dir, ".tiger", "stdreach-*.txt"))
	assert.NoError(err, "glob")
	assert.Equal(len(files), 1, "reach files written")
	reachFile.name = filepath.Base(files[0])
	reachFile.data, err = os.ReadFile(files[0])
	assert.NoError(err, "read the reach file")

	goleak.VerifyTestMain(m)
}
