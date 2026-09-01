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

// TestPluginDropsFacts covers the golangci driver's output policy.
//
// Goal: the analyzers the plugin hands golangci-lint never emit a fact
// diagnostic — golangci has no --show-facts channel and keeps one issue
// per line, so a surfaced fact would crowd a real finding off its line —
// while blocking diagnostics still fire.
func TestPluginDropsFacts(t *testing.T) {
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
				_, isFact := rules.ByFact(diagnostic.Category)
				assert.False(t, isFact)
				entry, known := rules.ByCategory(diagnostic.Category)
				require.True(t, known)
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

// TestPluginInstallsConfigFromSettings covers acceptance criterion 18 at
// the plugin's construction surface.
//
// Goal: a plugin built with a `config` setting naming a directory applies
// that directory's tiger.yaml — a scoped participle.allow entry silences
// TS-N14 in the scoped package and leaves it firing outside — and a
// directory whose tiger.yaml fails to load fails plugin construction with
// the one-line message.
func TestPluginInstallsConfigFromSettings(t *testing.T) {
	build, err := register.GetPlugin("tiger")
	require.NoError(t, err)

	corpus, err := filepath.Abs(filepath.Join("testdata", "corpus"))
	require.NoError(t, err)
	built, err := build(map[string]any{"config": filepath.Join("testdata", "withconfig")})
	require.NoError(t, err)
	analyzers, err := built.BuildAnalyzers()
	require.NoError(t, err)
	for _, analyzer := range analyzers {
		if analyzer.Name != "participle" {
			continue
		}
		results := analysistest.Run(t, corpus, analyzer,
			"smoke.example/plugin/scoped", "smoke.example/plugin/outside")
		fired := 0
		for _, result := range results {
			fired += len(result.Diagnostics)
		}
		assert.Equal(t, 1, fired)
	}

	// The config is process-global; leave nothing installed for other tests.
	t.Cleanup(func() {
		_, err := build(map[string]any{"config": t.TempDir()})
		require.NoError(t, err)
	})

	_, err = build(map[string]any{"config": filepath.Join("testdata", "badconfig")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tiger.yaml")
	assert.Contains(t, err.Error(), "frobnicate")
	assert.NotContains(t, err.Error(), "\n")
}
