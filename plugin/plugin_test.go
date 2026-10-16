package plugin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

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
	// Every registered analyzer, plus the one that keys the layer file.
	require.Len(t, analyzers, len(rules.Analyzers())+1)

	testdata, err := filepath.Abs(
		filepath.Join("..", "internal", "analyzers", "variant", "testdata"),
	)
	require.NoError(t, err)
	var variants, blocking int
	for _, analyzer := range analyzers {
		if analyzer.Name != "variant" {
			continue
		}
		for _, result := range analysistest.Run(quietRun{}, testdata, analyzer, "ts-v01") {
			for _, diagnostic := range result.Diagnostics {
				_, isFact := rules.ByFact(diagnostic.Category)
				assert.False(t, isFact)
				entry, known := rules.ByCategory(diagnostic.Category)
				require.True(t, known)
				variants++
				if entry.Severity == rules.SeverityBlocking {
					blocking++
				}
			}
		}
	}
	assert.Positive(t, variants)
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

// TestPluginInstallsLayerFileFromSettings covers TS-X03 at the plugin's
// construction surface.
//
// Goal: a plugin built with a `config` setting naming a directory installs
// that directory's tiger.layers.yaml, so the layers analyzer fails an import
// the entry doesn't list and passes one it does, reports a loaded package with
// no entry at its package clause, and a layer file that fails to load fails
// plugin construction with one line.
func TestPluginInstallsLayerFileFromSettings(t *testing.T) {
	build, err := register.GetPlugin("tiger")
	require.NoError(t, err)

	corpus, err := filepath.Abs(filepath.Join("testdata", "corpus"))
	require.NoError(t, err)
	built, err := build(map[string]any{"config": filepath.Join("testdata", "withlayers")})
	require.NoError(t, err)
	// The layer file is process-global; leave nothing installed for other tests.
	t.Cleanup(func() {
		_, err := build(map[string]any{"config": t.TempDir()})
		require.NoError(t, err)
	})
	analyzers, err := built.BuildAnalyzers()
	require.NoError(t, err)
	messages := []string{}
	for _, analyzer := range analyzers {
		if analyzer.Name != "layers" {
			continue
		}
		for _, result := range analysistest.Run(quietRun{}, corpus, analyzer,
			"smoke.example/plugin/layered") {
			for _, diagnostic := range result.Diagnostics {
				messages = append(messages, diagnostic.Message)
			}
		}
	}
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "doesn't list os, so this package can't import os")

	messages = []string{}
	for _, analyzer := range analyzers {
		if analyzer.Name != "layers" {
			continue
		}
		for _, result := range analysistest.Run(quietRun{}, corpus, analyzer,
			"smoke.example/plugin/unlisted") {
			for _, diagnostic := range result.Diagnostics {
				messages = append(messages, diagnostic.Message)
			}
		}
	}
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "package ./unlisted has no entry in tiger.layers.yaml")

	_, err = build(map[string]any{"config": filepath.Join("testdata", "badlayers")})
	require.ErrorContains(t, err,
		"tiger.layers.yaml: line 1: \"./layered\": \"strings\" is always allowed; remove it")
	assert.NotContains(t, err.Error(), "\n")
}

// TestPluginJudgesDirectoryEndingInDotTest covers TS-X03 under the plugin for
// a directory named like go test's synthesized main package.
//
// Goal: golangci-lint hands the layers analyzer a package in a directory whose
// name ends in .test, and since the package is not main the analyzer judges
// its imports, unlike tiger check, whose driver drops it.
func TestPluginJudgesDirectoryEndingInDotTest(t *testing.T) {
	build, err := register.GetPlugin("tiger")
	require.NoError(t, err)

	corpus, err := filepath.Abs(filepath.Join("testdata", "corpus"))
	require.NoError(t, err)
	built, err := build(map[string]any{"config": filepath.Join("testdata", "withlayers")})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := build(map[string]any{"config": t.TempDir()})
		require.NoError(t, err)
	})
	analyzers, err := built.BuildAnalyzers()
	require.NoError(t, err)
	messages := []string{}
	for _, analyzer := range analyzers {
		if analyzer.Name != "layers" {
			continue
		}
		for _, result := range analysistest.Run(quietRun{}, corpus, analyzer,
			"smoke.example/plugin/tool.test") {
			for _, diagnostic := range result.Diagnostics {
				messages = append(messages, diagnostic.Message)
			}
		}
	}
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "the \"./tool.test\" entry in tiger.layers.yaml doesn't list "+
		"os, so this package can't import os")
}

// TestPluginWithoutLayerFileLayersNothing covers TS-X03's opt-in contract at
// the plugin's construction surface.
//
// Goal: a plugin built over a directory with no tiger.layers.yaml clears the
// layer file an earlier construction installed, so the layers analyzer
// reports nothing.
func TestPluginWithoutLayerFileLayersNothing(t *testing.T) {
	build, err := register.GetPlugin("tiger")
	require.NoError(t, err)
	_, err = build(map[string]any{"config": filepath.Join("testdata", "withlayers")})
	require.NoError(t, err)

	built, err := build(map[string]any{"config": t.TempDir()})
	require.NoError(t, err)
	analyzers, err := built.BuildAnalyzers()
	require.NoError(t, err)
	corpus, err := filepath.Abs(filepath.Join("testdata", "corpus"))
	require.NoError(t, err)
	messages := []string{}
	for _, analyzer := range analyzers {
		if analyzer.Name != "layers" {
			continue
		}
		for _, result := range analysistest.Run(quietRun{}, corpus, analyzer,
			"smoke.example/plugin/layered") {
			for _, diagnostic := range result.Diagnostics {
				messages = append(messages, diagnostic.Message)
			}
		}
	}
	assert.Empty(t, messages)
}

// TestPluginLayerFileEditChangesTheAnalyzerNames covers TS-X03 under
// golangci-lint's issue cache, which keys a package's cached issues on the
// sorted names of the analyzers that ran, salted with the golangci-lint
// version string, the linters.settings block and go.mod, but not on the
// plugin binary or tiger.layers.yaml.
//
// Goal: two different layer files give analyzer sets whose names differ, so
// an edited file misses the cache rather than replaying stale results; the
// same file gives the same names, so the cache still hits; and the layers
// analyzer keeps its name, so issue text does not change.
func TestPluginLayerFileEditChangesTheAnalyzerNames(t *testing.T) {
	build, err := register.GetPlugin("tiger")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := build(map[string]any{"config": t.TempDir()})
		require.NoError(t, err)
	})
	names := func(dir string) []string {
		built, err := build(map[string]any{"config": dir})
		require.NoError(t, err)
		analyzers, err := built.BuildAnalyzers()
		require.NoError(t, err)
		found := []string{}
		for _, analyzer := range analyzers {
			found = append(found, analyzer.Name)
		}
		return found
	}
	edited := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(edited, "go.mod"),
		[]byte("module smoke.example/plugin\n\ngo 1.26\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(edited, "tiger.layers.yaml"),
		[]byte("\"./layered\": []\n"), 0o644))

	first := names(filepath.Join("testdata", "withlayers"))
	again := names(filepath.Join("testdata", "withlayers"))
	other := names(edited)
	assert.Equal(t, first, again)
	assert.NotEqual(t, first, other)
	assert.Contains(t, other, "layers")
}

// TestPluginEndsPatternBackedFindingsWithThePointer covers the golangci
// driver's half of the pattern pointer.
//
// Goal: a TS-C02 diagnostic reaching golangci-lint ends with the pointer to
// the patterns behind it, the same suffix tiger check prints.
func TestPluginEndsPatternBackedFindingsWithThePointer(t *testing.T) {
	build, err := register.GetPlugin("tiger")
	require.NoError(t, err)
	built, err := build(nil)
	require.NoError(t, err)
	analyzers, err := built.BuildAnalyzers()
	require.NoError(t, err)

	testdata, err := filepath.Abs(
		filepath.Join("..", "internal", "analyzers", "nogoroutine", "testdata"),
	)
	require.NoError(t, err)
	fired := 0
	for _, analyzer := range analyzers {
		if analyzer.Name != "nogoroutine" {
			continue
		}
		for _, result := range analysistest.Run(quietRun{}, testdata, analyzer, "ts-c02") {
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Category != "TS-C02" {
					continue
				}
				fired++
				assert.True(t, strings.HasSuffix(diagnostic.Message, rules.Pointer("TS-C02")))
			}
		}
	}
	assert.Positive(t, fired)
}

// TestPluginPointersMatchTheRegistryWithinTheCap covers the golangci
// driver's pointer for every pattern-backed boundedloop rule.
//
// Goal: each TS-S02 diagnostic (several patterns) and TS-S03 diagnostic
// (one pattern) that reaches golangci-lint ends with exactly the pointer the
// registry defines, and its body, pointer included, stays within 240
// characters.
func TestPluginPointersMatchTheRegistryWithinTheCap(t *testing.T) {
	const maxBodyRunes = 240
	build, err := register.GetPlugin("tiger")
	require.NoError(t, err)
	built, err := build(nil)
	require.NoError(t, err)
	analyzers, err := built.BuildAnalyzers()
	require.NoError(t, err)

	testdata, err := filepath.Abs(
		filepath.Join("..", "internal", "analyzers", "boundedloop", "testdata"),
	)
	require.NoError(t, err)
	fired := map[string]int{}
	for _, analyzer := range analyzers {
		if analyzer.Name != "boundedloop" {
			continue
		}
		results := analysistest.Run(quietRun{}, testdata, analyzer, "ts-s02", "ts-s03")
		for _, result := range results {
			for _, diagnostic := range result.Diagnostics {
				pointer := rules.Pointer(diagnostic.Category)
				if pointer == "" {
					continue
				}
				fired[diagnostic.Category]++
				assert.True(t, strings.HasSuffix(diagnostic.Message, pointer), diagnostic.Message)
				assert.Equal(t, 1, strings.Count(diagnostic.Message, "tiger pattern"))
				_, body, _ := strings.Cut(diagnostic.Message, ": ")
				runes := utf8.RuneCountInString(body)
				assert.LessOrEqual(t, runes, maxBodyRunes, diagnostic.Message)
			}
		}
	}
	assert.Positive(t, fired["TS-S02"])
	assert.Positive(t, fired["TS-S03"])
	assert.Equal(t, " — see `tiger pattern shutdown-loop`", rules.Pointer("TS-S03"))
	assert.Equal(t, " — see `tiger pattern --rule TS-S02`", rules.Pointer("TS-S02"))
}
