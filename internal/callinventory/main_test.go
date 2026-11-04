package callinventory_test

import (
	"os"
	"testing"
	"testing/fstest"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/callinventory"
)

// Loading one module first leaves the reach file in Go's build cache, so the
// tests that run in parallel read it instead of each type-checking the
// standard library at once.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tiger-reach-seed")
	assert.NoError(err, "temporary directory")
	assert.NoError(os.CopyFS(dir, fstest.MapFS{
		"go.mod":  {Data: []byte("module example.test/seed\n\ngo 1.22\n")},
		"seed.go": {Data: []byte("package seed\n")},
	}), "seed module")
	_, err = callinventory.Load(dir)
	assert.NoError(err, "generating the reach file")
	assert.NoError(os.RemoveAll(dir), "remove "+dir)
	os.Exit(m.Run())
}
