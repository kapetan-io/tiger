package layers_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/layers"
	"github.com/kapetan-io/tiger/internal/layerfile"
)

// TestCorpus runs the TS-X03 corpus through the analysistest driver with the
// corpus module's layer file installed.
//
// Goal: an unlisted impure subtree, in-module package, or third-party module,
// dotless or dotted, fires at its import, a package with no entry fires at its
// package clause, and listed imports, pure standard library and test files
// stay silent.
func TestCorpus(t *testing.T) {
	loaded, err := layerfile.Load(filepath.Join(analysistest.TestData(), "src", "ts-x03"))
	require.NoError(t, err)
	layerfile.Install(loaded)
	t.Cleanup(func() { layerfile.Install(nil) })
	analysistest.Run(t, analysistest.TestData(), layers.Analyzer, "ts-x03/...")
}
