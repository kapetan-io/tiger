package callinventory_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/callinventory"
)

var reachFile struct {
	name string
	data []byte
}

// Every test module gets a copy, so none of them type-checks the standard library.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tiger-reach-seed")
	assert.NoError(err, "temporary directory")
	defer func() { assert.NoError(os.RemoveAll(dir), "remove "+dir) }()
	assert.NoError(os.CopyFS(dir, fstest.MapFS{
		"go.mod":  {Data: []byte("module example.test/seed\n\ngo 1.22\n")},
		"seed.go": {Data: []byte("package seed\n")},
	}), "seed module")
	_, err = callinventory.Load(dir)
	assert.NoError(err, "generating the reach file")
	files, err := filepath.Glob(filepath.Join(dir, ".tiger", "stdreach-*.txt"))
	assert.NoError(err, "glob")
	assert.Equal(len(files), 1, "reach files written")
	reachFile.name = filepath.Base(files[0])
	reachFile.data, err = os.ReadFile(files[0])
	assert.NoError(err, "read the reach file")
	os.Exit(m.Run())
}
