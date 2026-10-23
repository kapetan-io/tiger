package ci_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContinuousIntegrationGuardsThePatterns reads the workflow that
// enforces acceptance criteria 6 and 7.
//
// Goal: CI checks the patterns tree with the built binary expecting empty
// output, runs the patterns under -race, and uploads the tiger binary next
// to the plugin.
func TestContinuousIntegrationGuardsThePatterns(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/ci.yml")
	require.NoError(t, err)
	workflow := string(raw)
	for _, want := range []string{
		"./bin/tiger check ./patterns/...", "test ! -s patterns.txt",
		"go test -race -count=1 ./patterns/...",
		"bin/tiger\n            bin/tiger-gcl\n",
	} {
		assert.Contains(t, workflow, want)
	}
}
