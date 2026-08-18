package plugin_test

import (
	"path/filepath"
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/rules"
)

// quietRun swallows analysistest's expectation mismatches; this test only
// consumes the collected diagnostics.
type quietRun struct{}

func (quietRun) Errorf(format string, args ...any) {}

// TestPluginDropsReportedFacts covers the golangci driver's severity
// policy.
//
// Goal: the analyzers the plugin hands golangci-lint never emit a
// reported-severity diagnostic — golangci has no --show-facts channel and
// keeps one issue per line, so a surfaced fact would crowd a real finding
// off its line — while blocking diagnostics still fire.
func TestPluginDropsReportedFacts(t *testing.T) {
	build, err := register.GetPlugin("tiger")
	require.NoError(t, err)
	built, err := build(nil)
	require.NoError(t, err)
	analyzers, err := built.BuildAnalyzers()
	require.NoError(t, err)
	require.Len(t, analyzers, len(rules.Analyzers()))

	testdata, err := filepath.Abs(
		filepath.Join("..", "internal", "analyzers", "effects", "testdata"),
	)
	require.NoError(t, err)
	var effects, blocking int
	for _, analyzer := range analyzers {
		if analyzer.Name != "effects" {
			continue
		}
		for _, result := range analysistest.Run(quietRun{}, testdata, analyzer, "ts-f01") {
			for _, diagnostic := range result.Diagnostics {
				entry, known := rules.ByCategory(diagnostic.Category)
				require.True(t, known)
				assert.NotEqual(t, rules.SeverityReported, entry.Severity)
				effects++
				if entry.Severity == rules.SeverityBlocking {
					blocking++
				}
			}
		}
	}
	assert.Positive(t, effects)
	assert.Positive(t, blocking)
}
