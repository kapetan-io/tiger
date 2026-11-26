package golangci_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/golangci"
)

type outcome struct {
	code   int
	stdout string
	stderr string
}

// run invokes tiger golangci once. The leading word keeps each call site
// reading as the command line it stands for.
func run(t *testing.T, args ...string) outcome {
	t.Helper()
	require.Equal(t, "golangci", args[0])
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := golangci.Run(args[1:], command.Streams{Stdout: stdout, Stderr: stderr})
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// copyFixture copies one fixture module into a fresh temp dir, so a test
// that reads it back cannot disturb the committed fixture.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(filepath.Join("testdata", "fixtures", name))))
	return dir
}

// readFile returns one file's bytes as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// moduleFile writes a minimal go.mod so tiger can read the module path.
type moduleFile struct {
	dir    string
	module string
}

func (m moduleFile) write(t *testing.T) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(m.dir, "go.mod"),
		[]byte("module "+m.module+"\n\ngo 1.26\n"), 0o644))
}

// TestGolangciInitThenVerifyRoundTrip covers the generation contract.
//
// Goal: the config --init generates from the registry passes verification
// unchanged, and a conforming config passes silently.
func TestGolangciInitThenVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "roundtrip.example/project"}.write(t)

	created := run(t, "golangci", "-C", dir, "--init")
	require.Equal(t, command.ExitClean, created.code)
	assert.Contains(t, created.stdout, ".golangci.yml")
	assert.Empty(t, created.stderr)

	generated, err := os.ReadFile(filepath.Join(dir, ".golangci.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(generated), "prefix(roundtrip.example/project)")
	assert.Contains(t, string(generated), "pkg: reflect")
	assert.NotContains(t, string(generated), "restrict")
	assert.NotContains(t, string(generated), "TS-P01")

	verified := run(t, "golangci", "-C", dir)
	assert.Equal(t, command.ExitClean, verified.code)
	assert.Empty(t, verified.stdout)
	assert.Empty(t, verified.stderr)
}

// TestGolangciInitRefusesExistingConfig covers the refusal contract.
//
// Goal: --init against a project that already has a config exits 2 without
// modifying the file — repairing a config is guided by verification
// findings, never by overwriting.
func TestGolangciInitRefusesExistingConfig(t *testing.T) {
	path := filepath.Join("testdata", "fixtures", "initexisting", ".golangci.yml")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	refused := run(
		t,
		"golangci",
		"-C",
		filepath.Join("testdata", "fixtures", "initexisting"),
		"--init",
	)
	assert.Equal(t, command.ExitOperational, refused.code)
	assert.Empty(t, refused.stdout)
	assert.Contains(t, refused.stderr, "already exists")

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

// TestGolangciMissingLinterFindsTheAutoRule covers the audit contract.
//
// Goal: a config missing a required linter produces a finding naming the
// auto rule that stopped being enforced and the change that restores it.
func TestGolangciMissingLinterFindsTheAutoRule(t *testing.T) {
	got := run(t, "golangci", "-C", filepath.Join("testdata", "fixtures", "missinglinter"))
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout, "TS-S04: funlen is missing from linters.enable")
	assert.Contains(t, got.stdout, "add funlen to linters.enable")
}

// TestGolangciDriftedSettingFindsTheAutoRule covers the audit contract.
//
// Goal: a baseline setting with a different value — stricter included, in
// v1 — produces a finding naming the auto rule and both values.
func TestGolangciDriftedSettingFindsTheAutoRule(t *testing.T) {
	got := run(t, "golangci", "-C", filepath.Join("testdata", "fixtures", "drifted"))
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout,
		"TS-S04: linters.settings.funlen.lines is 60, but tiger's baseline for the rule "+
			"\"hard limit of 70 lines per function\" is 70")
	assert.Contains(t, got.stdout, "stricter value also fails")
}

// TestGolangciAbsentConfigExitsTwo covers the operational contract.
//
// Goal: a project with no golangci-lint config is an operational failure,
// not a verification verdict.
func TestGolangciAbsentConfigExitsTwo(t *testing.T) {
	got := run(t, "golangci", "-C", filepath.Join("testdata", "fixtures", "noconfig"))
	assert.Equal(t, command.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "no golangci-lint config found")
}

// TestGolangciGeneratedConfigIsDeterministic covers constraint 4.
//
// Goal: two generations for the same module produce byte-identical configs.
func TestGolangciGeneratedConfigIsDeterministic(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	moduleFile{dir: first, module: "determinism.example/project"}.write(t)
	moduleFile{dir: second, module: "determinism.example/project"}.write(t)

	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", first, "--init").code)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", second, "--init").code)

	firstBytes, err := os.ReadFile(filepath.Join(first, ".golangci.yml"))
	require.NoError(t, err)
	secondBytes, err := os.ReadFile(filepath.Join(second, ".golangci.yml"))
	require.NoError(t, err)
	assert.Equal(t, firstBytes, secondBytes)
}

// TestGolangciPrintWritesBaselineToStdout covers acceptance criteria 20
// and 21.
//
// Goal: --print on a module with an existing config writes the generated
// baseline to stdout, leaves the tree untouched, exits 0, and its output is
// byte-identical to what --init writes for a fresh module of the same path.
func TestGolangciPrintWritesBaselineToStdout(t *testing.T) {
	existing := copyFixture(t, "initexisting")
	before := readFile(t, filepath.Join(existing, ".golangci.yml"))
	entriesBefore, err := os.ReadDir(existing)
	require.NoError(t, err)

	printed := run(t, "golangci", "-C", existing, "--print")
	assert.Equal(t, command.ExitClean, printed.code)
	assert.Empty(t, printed.stderr)
	assert.Contains(t, printed.stdout, "Generated by tiger golangci --init")
	assert.Equal(t, before, readFile(t, filepath.Join(existing, ".golangci.yml")))
	entriesAfter, err := os.ReadDir(existing)
	require.NoError(t, err)
	assert.Equal(t, entriesBefore, entriesAfter)

	fresh := t.TempDir()
	moduleFile{dir: fresh, module: readModule(t, existing)}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", fresh, "--init").code)
	assert.Equal(t, readFile(t, filepath.Join(fresh, ".golangci.yml")), printed.stdout)

	again := run(t, "golangci", "-C", existing, "--print")
	assert.Equal(t, printed, again)
}

// TestGolangciPrintWithoutModuleExitsTwo covers acceptance criterion 22.
//
// Goal: --print without a go.mod exits 2 with the message --init gives.
func TestGolangciPrintWithoutModuleExitsTwo(t *testing.T) {
	dir := t.TempDir()
	printed := run(t, "golangci", "-C", dir, "--print")
	assert.Equal(t, command.ExitOperational, printed.code)
	assert.Empty(t, printed.stdout)
	initialized := run(t, "golangci", "-C", dir, "--init")
	assert.Equal(t, command.ExitOperational, initialized.code)
	assert.Equal(t, initialized.stderr, printed.stderr)
	assert.Contains(t, printed.stderr, "reading module path")
}

// TestGolangciInitAndPrintTogetherExitTwo covers the flag exclusivity.
//
// Goal: --init with --print is an operational error that writes nothing.
func TestGolangciInitAndPrintTogetherExitTwo(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "both.example/project"}.write(t)
	got := run(t, "golangci", "-C", dir, "--init", "--print")
	assert.Equal(t, command.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "--init")
	assert.Contains(t, got.stderr, "--print")
	assert.NoFileExists(t, filepath.Join(dir, ".golangci.yml"))
}

// readModule returns the module path from dir's go.mod.
func readModule(t *testing.T, dir string) string {
	t.Helper()
	for _, line := range strings.Split(readFile(t, filepath.Join(dir, "go.mod")), "\n") {
		if module, found := strings.CutPrefix(line, "module "); found {
			return module
		}
	}
	require.FailNow(t, "go.mod has no module line")
	return ""
}

// TestShippedGolangciConfigDropsNewFromRev covers the removed recommendation.
//
// Goal: the config adopters copy, config/golangci.yml, no longer mentions
// --new-from-rev, which silently drops findings outside a diff (ADR-0022).
func TestShippedGolangciConfigDropsNewFromRev(t *testing.T) {
	shipped := readFile(t, filepath.Join("..", "..", "config", "golangci.yml"))
	assert.NotContains(t, shipped, "new-from-rev")
}

// shippedConfigIn copies config/golangci.yml, the template adopters copy,
// into a fresh module as its .golangci.yml and returns the module dir.
func shippedConfigIn(t *testing.T, edit func(string) string) string {
	t.Helper()
	dir := t.TempDir()
	moduleFile{dir: dir, module: "shipped.example/project"}.write(t)
	shipped := readFile(t, filepath.Join("..", "..", "config", "golangci.yml"))
	const placeholder, module = "prefix(myrepo)", "prefix(shipped.example/project)"
	config := edit(strings.Replace(shipped, placeholder, module, 1))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".golangci.yml"), []byte(config), 0o644))
	return dir
}

// TestShippedGolangciConfigPassesTheAudit covers the template adopters copy.
//
// Goal: config/golangci.yml carries the comments exclusion preset and
// passes tiger golangci with no findings.
func TestShippedGolangciConfigPassesTheAudit(t *testing.T) {
	shipped := readFile(t, filepath.Join("..", "..", "config", "golangci.yml"))
	assert.Contains(t, shipped, "      - comments\n")
	assert.NotContains(t, shipped, "package-comments")

	unchanged := func(config string) string { return config }
	got := run(t, "golangci", "-C", shippedConfigIn(t, unchanged))
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciMissingCommentsPresetFindsStutterRule covers the comments preset
// audit.
//
// Goal: a config whose exclusion presets lack comments, or that has no
// exclusions block at all, produces a TS-N10 finding naming the edit.
func TestGolangciMissingCommentsPresetFindsStutterRule(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(string) string
		want string
	}{
		{
			name: "presetDropped",
			edit: func(config string) string {
				return strings.Replace(config, "      - comments\n", "", 1)
			},
			want: "TS-N10: linters.exclusions.presets doesn't include comments, which " +
				"tiger's baseline for the rule \"no stutter\" needs — add comments to " +
				"linters.exclusions.presets (revive's exported rule demands doc " +
				"comments without it)",
		},
		{
			name: "noExclusionsBlock",
			edit: func(config string) string {
				return strings.Replace(config, "  exclusions:\n    generated: strict\n"+
					"    presets:\n      - comments\n      - common-false-positives\n", "", 1)
			},
			want: "TS-N10: linters.exclusions.presets isn't set",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := shippedConfigIn(t, test.edit)
			shipped := readFile(t, filepath.Join("..", "..", "config", "golangci.yml"))
			require.NotEqual(t, shipped, test.edit(shipped))
			got := run(t, "golangci", "-C", dir)
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Empty(t, got.stderr)
			assert.Contains(t, got.stdout, test.want)
		})
	}
}

// TestGolangciGeneratedConfigDropsDocCommentRules covers the generated baseline.
//
// Goal: the generated baseline excludes the comments preset and no longer
// asks revive for package-comments, and a config without package-comments
// raises no finding.
func TestGolangciGeneratedConfigDropsDocCommentRules(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "doccomments.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

	generated := readFile(t, filepath.Join(dir, ".golangci.yml"))
	assert.Contains(t, generated, "- comments")
	assert.NotContains(t, generated, "package-comments")

	got := run(t, "golangci", "-C", dir)
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
}

// TestGolangciGeneratedConfigUncapsIssueReports covers the generated baseline.
//
// Goal: the generated config lifts golangci-lint's per-linter and
// same-message caps, because a finding the cap hides is a finding nobody fixes.
func TestGolangciGeneratedConfigUncapsIssueReports(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "uncapped.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

	generated := readFile(t, filepath.Join(dir, ".golangci.yml"))
	assert.Contains(t, generated, "max-issues-per-linter: 0")
	assert.Contains(t, generated, "max-same-issues: 0")
}

// TestGolangciGeneratedConfigMergesSharedReviveRules covers the generated baseline.
//
// Goal: TS-N01 and TS-N10 both contribute to revive's rules list; the
// generated list holds both rules so each rule's audit passes, instead of
// the later one overwriting the earlier.
func TestGolangciGeneratedConfigMergesSharedReviveRules(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "revive.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

	generated := readFile(t, filepath.Join(dir, ".golangci.yml"))
	assert.Contains(t, generated, "- name: var-naming")
	assert.Contains(t, generated, "- name: exported")
}

// TestGolangciAuditPassesExtrasBeyondTheBaseline covers the audit contract.
//
// Goal: a config stricter or larger than the baseline still conforms: an
// extra linter, an extra list-of-maps element, an extra map key, and an
// integer written where the baseline holds a float (5 for 5.0) add no finding.
func TestGolangciAuditPassesExtrasBeyondTheBaseline(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "extras.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

	path := filepath.Join(dir, ".golangci.yml")
	config := readFile(t, path)
	for _, edit := range [][2]string{
		{"        - containedctx\n", "        - containedctx\n        - extralinter\n"},
		{
			"                - name: exported\n",
			"                - name: exported\n                - name: extrarule\n",
		},
		{
			"            package-average: 5.0\n",
			"            package-average: 5\n            extra-key: true\n",
		},
	} {
		require.Contains(t, config, edit[0])
		config = strings.Replace(config, edit[0], edit[1], 1)
	}
	require.NoError(t, os.WriteFile(path, []byte(config), 0o644))

	got := run(t, "golangci", "-C", dir)
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciGeneratedConfigEnablesTheGocriticChecks covers the generated baseline.
//
// Goal: TS-M03 sets the two gocritic size thresholds, and gocritic runs a
// check only when it is enabled, so the generated config also enables
// hugeParam and rangeValCopy. Without that, golangci-lint warns that settings
// were given for a disabled check and reports no by-value copies.
func TestGolangciGeneratedConfigEnablesTheGocriticChecks(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "gocritic.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

	var document struct {
		Linters struct {
			Settings struct {
				Gocritic struct {
					EnabledChecks []string `yaml:"enabled-checks"`
				} `yaml:"gocritic"`
			} `yaml:"settings"`
		} `yaml:"linters"`
	}
	generated := readFile(t, filepath.Join(dir, ".golangci.yml"))
	require.NoError(t, yaml.Unmarshal([]byte(generated), &document))
	assert.ElementsMatch(t, []string{"hugeParam", "rangeValCopy"},
		document.Linters.Settings.Gocritic.EnabledChecks)
}

// TestGolangciGocriticThresholdsWithoutTheChecksFindTheAutoRule covers the audit contract.
//
// Goal: a config that sets the TS-M03 size thresholds but leaves hugeParam
// and rangeValCopy off is reported as drift from the baseline.
func TestGolangciGocriticThresholdsWithoutTheChecksFindTheAutoRule(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "unchecked.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

	path := filepath.Join(dir, ".golangci.yml")
	config := readFile(t, path)
	start := strings.Index(config, "enabled-checks:")
	require.NotEqual(t, -1, start)
	end := start + strings.Index(config[start:], "settings:")
	require.Greater(t, end, start)
	require.NoError(t, os.WriteFile(path, []byte(config[:start]+config[end:]), 0o644))

	got := run(t, "golangci", "-C", dir)
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "TS-M03: linters.settings.gocritic.enabled-checks isn't set")
	assert.Contains(t, got.stdout, "to [hugeParam, rangeValCopy]")
}

// TestGolangciGocriticChecksMissingOneFindsTheAutoRule covers the partial list.
//
// Goal: a config that enables hugeParam but not rangeValCopy is reported
// with a finding naming rangeValCopy, and not hugeParam.
func TestGolangciGocriticChecksMissingOneFindsTheAutoRule(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "partial.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)
	path := filepath.Join(dir, ".golangci.yml")
	config := strings.Replace(readFile(t, path), "                - rangeValCopy\n", "", 1)
	require.NoError(t, os.WriteFile(path, []byte(config), 0o644))

	got := run(t, "golangci", "-C", dir)
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout,
		"TS-M03: linters.settings.gocritic.enabled-checks doesn't include rangeValCopy")
	assert.NotContains(t, got.stdout, "include hugeParam")
}

// TestGolangciGocriticChecksEnabledAnotherWayPassTheAudit covers the audit contract.
//
// Goal: a config that turns hugeParam and rangeValCopy on through
// enable-all or the performance tag, instead of enabled-checks, conforms.
// Listing enabled-checks beside enable-all would make golangci-lint fail.
func TestGolangciGocriticChecksEnabledAnotherWayPassTheAudit(t *testing.T) {
	for _, test := range []struct {
		name string
		with string
	}{
		{name: "enableAll", with: "enable-all: true\n"},
		{name: "performanceTag", with: "enabled-tags:\n                - performance\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			moduleFile{dir: dir, module: "another.example/project"}.write(t)
			require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

			path := filepath.Join(dir, ".golangci.yml")
			config := readFile(t, path)
			const checks = "            enabled-checks:\n                - hugeParam\n" +
				"                - rangeValCopy\n"
			require.Contains(t, config, checks)
			config = strings.Replace(config, checks, "            "+test.with, 1)
			require.NoError(t, os.WriteFile(path, []byte(config), 0o644))

			got := run(t, "golangci", "-C", dir)
			assert.Equal(t, command.ExitClean, got.code)
			assert.Empty(t, got.stdout)
		})
	}
}

// TestGolangciGeneratedConfigFlagsStructsLargerThan64Bytes covers the generated baseline.
//
// Goal: TS-M03 says larger than 64 bytes, and gocritic flags a size at or
// above its threshold, so the generated thresholds are 65.
func TestGolangciGeneratedConfigFlagsStructsLargerThan64Bytes(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "threshold.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

	generated := readFile(t, filepath.Join(dir, ".golangci.yml"))
	assert.Contains(t, generated, "hugeParam:\n                    sizeThreshold: 65")
	assert.Contains(t, generated, "rangeValCopy:\n                    sizeThreshold: 65")
}

// TestGolangciGocriticChecksSwitchedOffFindTheAutoRule covers the audit contract.
//
// Goal: a config that turns hugeParam or rangeValCopy off is reported, however
// the checks were turned on, because golangci-lint applies the disabled lists
// after every enable. Disabling one check is reported for that check only.
func TestGolangciGocriticChecksSwitchedOffFindTheAutoRule(t *testing.T) {
	const checks = "            enabled-checks:\n                - hugeParam\n" +
		"                - rangeValCopy\n"
	const offChecks = "            disabled-checks:\n                - hugeParam\n" +
		"                - rangeValCopy\n"
	bothDisabled := []string{
		"disabled-checks lists hugeParam", "disabled-checks lists rangeValCopy",
	}
	const offHuge = "            disabled-checks:\n                - hugeParam\n"
	const offTag = "            disabled-tags:\n                - performance\n"
	for _, test := range []struct {
		name   string
		config string
		want   []string
		absent []string
	}{
		{
			name:   "enableAllDisabledChecks",
			config: "            enable-all: true\n" + offChecks,
			want:   bothDisabled,
		},
		{
			name:   "performanceTagDisabledChecks",
			config: "            enabled-tags:\n                - performance\n" + offChecks,
			want:   bothDisabled,
		},
		{
			name:   "enableAllDisabledTag",
			config: "            enable-all: true\n" + offTag,
			want:   []string{"disabled-tags lists performance"},
		},
		{
			name:   "enabledChecksDisabledTag",
			config: checks + offTag,
			want:   []string{"disabled-tags lists performance"},
		},
		{
			name:   "oneCheckDisabled",
			config: "            enable-all: true\n" + offHuge,
			want:   []string{"disabled-checks lists hugeParam"},
			absent: []string{"rangeValCopy"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			moduleFile{dir: dir, module: "switchedoff.example/project"}.write(t)
			require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

			path := filepath.Join(dir, ".golangci.yml")
			config := readFile(t, path)
			require.Contains(t, config, checks)
			config = strings.Replace(config, checks, test.config, 1)
			require.NoError(t, os.WriteFile(path, []byte(config), 0o644))

			got := run(t, "golangci", "-C", dir)
			assert.Equal(t, command.ExitFindings, got.code)
			for _, want := range test.want {
				assert.Contains(t, got.stdout, "TS-M03: linters.settings.gocritic."+want)
			}
			for _, absent := range test.absent {
				assert.NotContains(t, got.stdout, absent)
			}

			findings, err := golangci.Verify(dir)
			require.NoError(t, err)
			require.Len(t, findings, len(test.want))
			for _, finding := range findings {
				checkStyle(t, finding)
			}
		})
	}
}

// TestGolangciGeneratedConfigBansEveryClockRead covers the clock bans.
//
// Goal: --init emits a ban for each way to read the clock, and each ban's
// message tells the author to take the time, timer or ticker from an
// injected clock.
func TestGolangciGeneratedConfigBansEveryClockRead(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "clockbans.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)
	generated := readFile(t, filepath.Join(dir, ".golangci.yml"))

	for _, test := range []struct {
		name string
		call string
		rule string
	}{
		{name: "now", call: "Now", rule: "TS-T01"},
		{name: "since", call: "Since", rule: "TS-T01"},
		{name: "until", call: "Until", rule: "TS-T01"},
		{name: "after", call: "After", rule: "TS-C05"},
		{name: "tick", call: "Tick", rule: "TS-T01"},
		{name: "newTimer", call: "NewTimer", rule: "TS-T01"},
		{name: "newTicker", call: "NewTicker", rule: "TS-T01"},
		{name: "afterFunc", call: "AfterFunc", rule: "TS-T01"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Contains(t, generated, `pattern: ^time\.`+test.call+`$`)
			assert.Regexp(t, `msg: `+test.rule+`\..*injected clock.*\n\s+pattern: \^time\\\.`+
				test.call+`\$\n`, generated)
		})
	}
	assert.NotContains(t, generated, "a timer you stop")
}

// TestGolangciAuditRequiresEveryBaselineBan covers the "at least these" audit.
//
// Goal: a config holding every baseline ban plus its own, with its own
// wording, passes; a config missing baseline bans fails with a finding that
// names each missing ban.
func TestGolangciAuditRequiresEveryBaselineBan(t *testing.T) {
	unchanged := func(config string) string { return config }
	extra := func(config string) string {
		return strings.Replace(config, "        - pattern: '^panic$'\n",
			"        - pattern: '^os\\.Exit$'\n          msg: Our own ban, worded our own way.\n"+
				"        - pattern: '^panic$'\n", 1)
	}
	reworded := func(config string) string {
		return strings.Replace(config, "TS-T01. Take the time", "Use our clock. Take the time", 1)
	}
	dropped := func(config string) string {
		for _, call := range []string{"Since", "NewTicker"} {
			ban := regexp.MustCompile(`        - pattern: '\^time\\\.` + call + `\$'\n.*\n`)
			config = ban.ReplaceAllString(config, "")
		}
		return config
	}
	for _, test := range []struct {
		name string
		edit func(string) string
	}{
		{name: "baseline", edit: unchanged},
		{name: "extraBan", edit: extra},
		{name: "ownWording", edit: reworded},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "golangci", "-C", shippedConfigIn(t, test.edit))
			assert.Equal(t, command.ExitClean, got.code)
			assert.Empty(t, got.stdout)
		})
	}

	got := run(t, "golangci", "-C", shippedConfigIn(t, dropped))
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, `linters.settings.forbidigo.forbid doesn't include `+
		`^time\.Since$, ^time\.NewTicker$`)
}

// TestGolangciAuditRejectsBareStringBan covers the audit's ban identity.
//
// Goal: a forbid entry written as a bare string is not a baseline ban, so a
// config holding time.Now only that way fails with a finding naming it.
func TestGolangciAuditRejectsBareStringBan(t *testing.T) {
	bareString := func(config string) string {
		ban := regexp.MustCompile(`- pattern: ('\^time\\\.Now\$')\n\s+msg: [^\n]*\n`)
		return ban.ReplaceAllString(config, "- $1\n")
	}
	shipped := readFile(t, filepath.Join("..", "..", "config", "golangci.yml"))
	require.NotEqual(t, shipped, bareString(shipped))

	got := run(t, "golangci", "-C", shippedConfigIn(t, bareString))
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, `doesn't include ^time\.Now$`)
}

// TestGolangciGeneratedConfigLeavesFieldAlignmentOff covers the generated baseline.
//
// Goal: govet keeps enable-all for the shadow check TS-S14 needs, and names
// fieldalignment as its one exception, since no Tiger rule orders struct
// fields by size.
func TestGolangciGeneratedConfigLeavesFieldAlignmentOff(t *testing.T) {
	dir := t.TempDir()
	moduleFile{dir: dir, module: "aligned.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

	var document struct {
		Linters struct {
			Settings struct {
				Govet struct {
					EnableAll bool     `yaml:"enable-all"`
					Disable   []string `yaml:"disable"`
				} `yaml:"govet"`
			} `yaml:"settings"`
		} `yaml:"linters"`
	}
	generated := readFile(t, filepath.Join(dir, ".golangci.yml"))
	require.NoError(t, yaml.Unmarshal([]byte(generated), &document))
	assert.True(t, document.Linters.Settings.Govet.EnableAll)
	assert.Equal(t, []string{"fieldalignment"}, document.Linters.Settings.Govet.Disable)
}

// TestGolangciGovetDisableOtherThanFieldAlignmentFindsTheAutoRule covers the audit contract.
//
// Goal: a config must leave govet's disable list at exactly fieldalignment.
// Dropping the line turns fieldalignment back on, and naming another
// analyzer, such as shadow, switches off a check a Tiger rule needs.
func TestGolangciGovetDisableOtherThanFieldAlignmentFindsTheAutoRule(t *testing.T) {
	for _, test := range []struct {
		name    string
		replace string
		want    string
	}{
		{
			name:    "missing",
			replace: "",
			want:    "TS-S14: linters.settings.govet.disable isn't set",
		},
		{
			name: "shadow added",
			replace: "            disable:\n" +
				"                - fieldalignment\n                - shadow\n",
			want: "TS-S14: linters.settings.govet.disable is [fieldalignment, shadow]",
		},
		{
			name:    "empty",
			replace: "            disable: []\n",
			want:    "TS-S14: linters.settings.govet.disable is []",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			moduleFile{dir: dir, module: "govet.example/project"}.write(t)
			require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)

			path := filepath.Join(dir, ".golangci.yml")
			config := readFile(t, path)
			const line = "            disable:\n                - fieldalignment\n"
			require.Contains(t, config, line)
			config = strings.Replace(config, line, test.replace, 1)
			require.NoError(t, os.WriteFile(path, []byte(config), 0o644))

			got := run(t, "golangci", "-C", dir)
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Contains(t, got.stdout, test.want)
		})
	}
}

// initThenEdit writes the baseline with --init, applies each replacement once,
// and returns the module dir.
func initThenEdit(t *testing.T, edits ...[2]string) string {
	t.Helper()
	dir := t.TempDir()
	moduleFile{dir: dir, module: "keys.example/project"}.write(t)
	require.Equal(t, command.ExitClean, run(t, "golangci", "-C", dir, "--init").code)
	path := filepath.Join(dir, ".golangci.yml")
	config := readFile(t, path)
	for _, edit := range edits {
		require.Contains(t, config, edit[0])
		config = strings.Replace(config, edit[0], edit[1], 1)
	}
	require.NoError(t, os.WriteFile(path, []byte(config), 0o644))
	return dir
}

// TestGolangciAuditReportsKeysThatHideFindings covers the whole-file comparison.
//
// Goal: outside the settings blocks, the enable lists, linters.default,
// issues.uniq-by-line and the exclusion rules, a key that differs from the
// baseline is reported by name, since keys such as exclusions.paths and
// run.tests hide findings while every linter still looks turned on.
func TestGolangciAuditReportsKeysThatHideFindings(t *testing.T) {
	for _, test := range []struct {
		name string
		edit [2]string
		want string
	}{
		{
			name: "exclusionPathsAdded",
			edit: [2]string{
				"    exclusions:\n",
				"    exclusions:\n        paths:\n            - internal/\n",
			},
			want: "linters.exclusions.paths is set, but tiger's baseline doesn't set it — " +
				"remove linters.exclusions.paths",
		},
		{
			name: "testsSkipped",
			edit: [2]string{"version: \"2\"\n", "version: \"2\"\nrun:\n    tests: false\n"},
			want: "run.tests is set, but tiger's baseline doesn't set it",
		},
		{
			name: "presetAdded",
			edit: [2]string{
				"            - comments\n",
				"            - comments\n            - std-error-handling\n",
			},
			want: "TS-N10: linters.exclusions.presets lists std-error-handling, which tiger's baseline " +
				"doesn't — remove std-error-handling from " +
				"linters.exclusions.presets",
		},
		{
			name: "laxGeneratedCode",
			edit: [2]string{"    exclusions:\n", "    exclusions:\n        generated: lax\n"},
			want: "linters.exclusions.generated is set, but tiger's baseline doesn't set it",
		},
		{
			name: "issueCapDropped",
			edit: [2]string{"    max-same-issues: 0\n", ""},
			want: "issues.max-same-issues isn't set, but tiger's baseline sets it — set " +
				"issues.max-same-issues to 0",
		},
		{
			name: "issueCapChanged",
			edit: [2]string{"    max-same-issues: 0\n", "    max-same-issues: 3\n"},
			want: "issues.max-same-issues is 3, but tiger's baseline is 0 — set it to 0",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "golangci", "-C", initThenEdit(t, test.edit))
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Contains(t, got.stdout, test.want)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestGolangciAuditPassesKeysThatHideNothing covers the values the comparison
// lets through.
//
// Goal: generated: strict is golangci-lint's own default, and the
// common-false-positives preset drops only gosec checks the baseline never
// enables, so neither is reported.
func TestGolangciAuditPassesKeysThatHideNothing(t *testing.T) {
	got := run(t, "golangci", "-C", initThenEdit(t, [2]string{
		"    exclusions:\n        presets:\n            - comments\n",
		"    exclusions:\n        generated: strict\n        presets:\n            - comments\n" +
			"            - common-false-positives\n",
	}))
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciOffLeavesNamedLintersUnreported covers --off.
//
// Goal: a linter named in --off is not reported missing from linters.enable,
// one not named still is, and the flag takes a comma-separated list or
// repeats.
func TestGolangciOffLeavesNamedLintersUnreported(t *testing.T) {
	dir := initThenEdit(t,
		[2]string{"        - wrapcheck\n", ""},
		[2]string{"        - lll\n", ""},
		[2]string{"        - varnamelen\n", ""},
	)

	got := run(t, "golangci", "-C", dir, "--off", "wrapcheck,lll")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Equal(t, "TS-N02: varnamelen is missing from linters.enable, so nothing enforces the "+
		"rule \"no invented abbreviations\" — add varnamelen to linters.enable\n"+
		"tiger: 1 auto rules unenforced\n", got.stdout)

	got = run(t, "golangci", "-C", dir, "--off", "wrapcheck,lll", "--off", "varnamelen")
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciOffStillAuditsSettingsOfTurnedOnLinters covers --off on a
// linter that is turned on.
//
// Goal: naming a turned-on linter in --off does not skip its settings, so a
// drifted setting is still reported.
func TestGolangciOffStillAuditsSettingsOfTurnedOnLinters(t *testing.T) {
	dir := initThenEdit(t, [2]string{"            lines: 70\n", "            lines: 90\n"})
	got := run(t, "golangci", "-C", dir, "--off", "funlen")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "TS-S04: linters.settings.funlen.lines is 90")
}

// TestGolangciOffRejectsAName covers --off input the audit refuses.
//
// Goal: a name that is not a linter in the baseline, such as a typo, exits 2
// before the audit runs, so it cannot hide a real missing linter; --off
// with --init or --print exits 2 too.
func TestGolangciOffRejectsAName(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "typo",
			args: []string{"--off", "wrapchek"},
			want: "tiger golangci: --off names \"wrapchek\", which is not a linter in tiger's " +
				"baseline — name one of ",
		},
		{
			name: "empty",
			args: []string{"--off", "lll,"},
			want: "tiger golangci: --off names \"\", which is not a linter in tiger's baseline",
		},
		{
			name: "withPrint",
			args: []string{"--off", "lll", "--print"},
			want: "tiger golangci: --off applies only to the audit",
		},
		{
			name: "withInit",
			args: []string{"--off", "lll", "--init"},
			want: "tiger golangci: --off applies only to the audit",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := initThenEdit(t)
			got := run(t, append([]string{"golangci", "-C", dir}, test.args...)...)
			assert.Equal(t, command.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Contains(t, got.stderr, test.want)
		})
	}
}

// TestGolangciAuditPassesAConfigThatDiffersOnlyWhereAllowed covers a config
// cut down for adoption.
//
// Goal: a config that differs from the baseline only in its enable lists,
// linters.default, issues.uniq-by-line and its exclusion rules passes, with
// the linters it has not turned on yet named in --off. This is the shape a
// repository adopting the baseline one linter at a time commits first.
func TestGolangciAuditPassesAConfigThatDiffersOnlyWhereAllowed(t *testing.T) {
	dir := initThenEdit(t,
		[2]string{"    default: standard\n", "    default: none\n"},
		[2]string{"    max-same-issues: 0\n", "    max-same-issues: 0\n    uniq-by-line: false\n"},
		[2]string{"        - wrapcheck\n", "        - errcheck\n"},
		[2]string{"        - lll\n", ""},
		[2]string{"        - gofumpt\n", ""},
		[2]string{"        presets:\n", "        rules:\n" +
			"            # waiver: the generated client is not ours to fix.\n" +
			"            - path: internal/client/\n" +
			"              linters:\n                - mnd\n" +
			"        presets:\n"},
	)

	got := run(t, "golangci", "-C", dir,
		"--off", "wrapcheck,lll,gofumpt",
		"--off", "govet,ineffassign,staticcheck")
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciAuditPassesGeneratedCodeExcludedEitherWay covers the values of
// linters.exclusions.generated the comparison lets through.
//
// Goal: generated: disable lints generated files too, so it hides nothing
// and passes, as strict does.
func TestGolangciAuditPassesGeneratedCodeExcludedEitherWay(t *testing.T) {
	for _, value := range []string{"strict", "disable"} {
		t.Run(value, func(t *testing.T) {
			got := run(t, "golangci", "-C", initThenEdit(t, [2]string{
				"    exclusions:\n",
				"    exclusions:\n        generated: " + value + "\n",
			}))
			assert.Equal(t, command.ExitClean, got.code)
			assert.Empty(t, got.stdout)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestGolangciAuditReportsMoreKeysThatHideFindings covers the whole-file
// comparison beyond linters.exclusions and run.
//
// Goal: linters.disable switches a linter off while it is still listed in
// linters.enable, and formatters.exclusions.paths hides formatting findings,
// so each is reported by name.
func TestGolangciAuditReportsMoreKeysThatHideFindings(t *testing.T) {
	for _, test := range []struct {
		name string
		edit [2]string
		want string
	}{
		{
			name: "lintersDisable",
			edit: [2]string{
				"    default: standard\n",
				"    default: standard\n    disable:\n        - wrapcheck\n",
			},
			want: "TS-E03: linters.disable lists wrapcheck, which switches off the rule " +
				"\"wrap errors crossing package boundaries\" — remove wrapcheck from linters.disable",
		},
		{
			name: "formatterPathsExcluded",
			edit: [2]string{
				"formatters:\n",
				"formatters:\n    exclusions:\n        paths:\n            - internal/\n",
			},
			want: "formatters.exclusions.paths is set, but tiger's baseline doesn't set it",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "golangci", "-C", initThenEdit(t, test.edit))
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Contains(t, got.stdout, test.want)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestGolangciOffStillReportsAPresetThatHidesFindings covers --off on the
// linter whose rule owns linters.exclusions.presets.
//
// Goal: with revive off and named in --off, the comments preset is no longer
// needed, but an extra preset still hides other linters' findings and is
// still reported.
func TestGolangciOffStillReportsAPresetThatHidesFindings(t *testing.T) {
	dir := initThenEdit(t,
		[2]string{"        - revive\n", ""},
		[2]string{"            - comments\n", "            - std-error-handling\n"},
	)
	got := run(t, "golangci", "-C", dir, "--off", "revive")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Equal(t, "TS-N10: linters.exclusions.presets lists std-error-handling, which tiger's "+
		"baseline doesn't — remove std-error-handling from "+
		"linters.exclusions.presets\n"+
		"tiger: 1 auto rules unenforced\n", got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciOffStillAuditsSettingsOfDefaultLinters covers --off on a linter
// turned on by linters.default rather than linters.enable.
//
// Goal: govet is on through default: standard, so naming it in --off does not
// skip its settings, and a dropped govet.disable is still reported.
func TestGolangciOffStillAuditsSettingsOfDefaultLinters(t *testing.T) {
	dir := initThenEdit(t, [2]string{
		"            disable:\n                - fieldalignment\n", "",
	})
	got := run(t, "golangci", "-C", dir, "--off", "govet")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "TS-S14: linters.settings.govet.disable isn't set")
	assert.Empty(t, got.stderr)
}

// TestGolangciAuditNamesTheKeysOfABlockLeftEmptyOrMisshapen covers blocks
// whose body is missing or the wrong shape.
//
// Goal: a block with no body is read as absent, so the audit names each
// baseline key under it once, on one line; a block written as a list says so
// on one line; the TS-N10 presets finding is not repeated.
func TestGolangciAuditNamesTheKeysOfABlockLeftEmptyOrMisshapen(t *testing.T) {
	for _, test := range []struct {
		name string
		edit [2]string
		want string
	}{
		{
			name: "issuesWithNoBody",
			edit: [2]string{
				"issues:\n    max-issues-per-linter: 0\n    max-same-issues: 0\n",
				"issues:\n",
			},
			want: "issues.max-issues-per-linter isn't set, but tiger's baseline sets it — set " +
				"issues.max-issues-per-linter to 0\n" +
				"issues.max-same-issues isn't set, but tiger's baseline sets it — set " +
				"issues.max-same-issues to 0\n" +
				"tiger: 2 auto rules unenforced\n",
		},
		{
			name: "exclusionsWithNoBody",
			edit: [2]string{
				"    exclusions:\n        presets:\n            - comments\n",
				"    exclusions:\n",
			},
			want: "TS-N10: linters.exclusions.presets isn't set, and tiger's baseline for the " +
				"rule \"no stutter\" needs it — set linters.exclusions.presets to [comments] " +
				"(revive's exported rule demands doc comments without it)\n" +
				"tiger: 1 auto rules unenforced\n",
		},
		{
			name: "issuesAsAList",
			edit: [2]string{
				"issues:\n    max-issues-per-linter: 0\n    max-same-issues: 0\n",
				"issues: []\n",
			},
			want: "issues isn't laid out as in tiger's baseline, which holds keys under it — " +
				"copy issues from tiger golangci --print\n" +
				"tiger: 1 auto rules unenforced\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "golangci", "-C", initThenEdit(t, test.edit))
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Equal(t, test.want, got.stdout)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestGolangciAuditReadsAnUnquotedVersion covers version: 2 written as a
// number.
//
// Goal: golangci-lint reads version: 2 the same as version: "2", so the audit
// does not report it.
func TestGolangciAuditReadsAnUnquotedVersion(t *testing.T) {
	got := run(t, "golangci", "-C", initThenEdit(t, [2]string{"version: \"2\"\n", "version: 2\n"}))
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciOffTrimsSpacesAroundNames covers --off "a, b".
//
// Goal: a space after a comma is not part of the linter name.
func TestGolangciOffTrimsSpacesAroundNames(t *testing.T) {
	dir := initThenEdit(t,
		[2]string{"        - wrapcheck\n", ""},
		[2]string{"        - lll\n", ""},
	)
	got := run(t, "golangci", "-C", dir, "--off", "wrapcheck, lll")
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciAuditKeepsEveryKeyFindingOnOneLine covers values the audit
// can't print on one line.
//
// Goal: a key holding a list of blocks is reported on one line, and an
// exclusions block of the wrong shape gets only the TS-N10 finding for the
// presets the rule owns, not a second line for the block.
func TestGolangciAuditKeepsEveryKeyFindingOnOneLine(t *testing.T) {
	for _, test := range []struct {
		name string
		edit [2]string
		want string
	}{
		{
			name: "listOfBlocks",
			edit: [2]string{
				"    max-same-issues: 0\n",
				"    max-same-issues:\n        - a: 1\n        - b: 2\n",
			},
			want: "issues.max-same-issues is a block of keys, but tiger's baseline is 0 — " +
				"set it to 0\n" +
				"tiger: 1 auto rules unenforced\n",
		},
		{
			name: "exclusionsAsAList",
			edit: [2]string{
				"    exclusions:\n        presets:\n            - comments\n",
				"    exclusions: []\n",
			},
			want: "TS-N10: linters.exclusions.presets isn't set, and tiger's baseline for the " +
				"rule \"no stutter\" needs it — set linters.exclusions.presets to [comments] " +
				"(revive's exported rule demands doc comments without it)\n" +
				"tiger: 1 auto rules unenforced\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "golangci", "-C", initThenEdit(t, test.edit))
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Equal(t, test.want, got.stdout)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestGolangciDisabledLinterFindsItsAutoRules covers linters.disable.
//
// Goal: a baseline linter listed under linters.disable is reported once for
// each auto rule it enforces, led by that rule's code, like a linter missing
// from linters.enable. A linter named in --off, or one no auto rule uses,
// may be listed.
func TestGolangciDisabledLinterFindsItsAutoRules(t *testing.T) {
	const disable = "    default: standard\n    disable:\n        - errcheck\n        - unused\n" +
		"        - lll\n"
	dir := initThenEdit(t, [2]string{"    default: standard\n", disable})

	got := run(t, "golangci", "-C", dir, "--off", "lll")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Equal(t, "TS-E01: linters.disable lists errcheck, which switches off the rule "+
		"\"every error is handled\" — remove errcheck from linters.disable\n"+
		"TS-S20: linters.disable lists errcheck, which switches off the rule "+
		"\"type assertions are checked via errcheck\" — remove errcheck from linters.disable\n"+
		"tiger: 2 auto rules unenforced\n", got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciDisabledLinterWrittenAsTextFindsItsAutoRules covers a
// linters.disable written as one string, which golangci-lint splits on commas.
//
// Goal: each baseline linter in the string is reported under its auto rules,
// and a linters.disable holding a block is reported by key.
func TestGolangciDisabledLinterWrittenAsTextFindsItsAutoRules(t *testing.T) {
	for _, test := range []struct {
		name    string
		disable string
		want    string
	}{
		{
			name:    "commaSeparated",
			disable: "    disable: wrapcheck, errcheck\n",
			want: "TS-E01: linters.disable lists errcheck, which switches off the rule " +
				"\"every error is handled\" — remove errcheck from linters.disable\n" +
				"TS-E03: linters.disable lists wrapcheck, which switches off the rule " +
				"\"wrap errors crossing package boundaries\" — remove wrapcheck from " +
				"linters.disable\n" +
				"TS-S20: linters.disable lists errcheck, which switches off the rule " +
				"\"type assertions are checked via errcheck\" — remove errcheck from " +
				"linters.disable\n" +
				"tiger: 3 auto rules unenforced\n",
		},
		{
			name:    "block",
			disable: "    disable:\n        errcheck: true\n",
			want: "linters.disable.errcheck is set, but tiger's baseline doesn't set it — " +
				"remove linters.disable.errcheck\n" +
				"tiger: 1 auto rules unenforced\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := initThenEdit(t, [2]string{
				"    default: standard\n", "    default: standard\n" + test.disable,
			})
			got := run(t, "golangci", "-C", dir)
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Equal(t, test.want, got.stdout)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestGolangciDisabledFormatterFindsItsAutoRule covers a formatter listed
// under linters.disable.
//
// Goal: golangci-lint refuses to load such a config, so the audit reports it
// under the formatter's auto rule.
func TestGolangciDisabledFormatterFindsItsAutoRule(t *testing.T) {
	dir := initThenEdit(t, [2]string{
		"    default: standard\n", "    default: standard\n    disable:\n        - gofumpt\n",
	})

	got := run(t, "golangci", "-C", dir)
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, ": linters.disable lists gofumpt, a formatter, so "+
		"golangci-lint refuses to load the config — remove gofumpt from linters.disable\n")
	assert.Empty(t, got.stderr)
}

// TestGolangciOffLeavesADisabledStandardLinterUnreported covers a linter in
// golangci-lint's standard set, which linters.default turns on unless
// linters.disable lists it.
//
// Goal: a standard linter both disabled and named in --off has its settings
// left unaudited, like a linter left out of linters.enable.
func TestGolangciOffLeavesADisabledStandardLinterUnreported(t *testing.T) {
	dir := initThenEdit(t,
		[2]string{
			"    default: standard\n",
			"    default: standard\n    disable:\n        - errcheck\n",
		},
		[2]string{
			"        errcheck:\n            check-blank: true\n" +
				"            check-type-assertions: true\n",
			"",
		},
	)

	got := run(t, "golangci", "-C", dir, "--off", "errcheck")
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestGolangciOffStillReportsAMisshapenExclusionsBlock covers a misshapen
// linters.exclusions block whose only rule-owned key belongs to a linter named
// in --off.
//
// Goal: with that rule skipped, the block is still reported.
func TestGolangciOffStillReportsAMisshapenExclusionsBlock(t *testing.T) {
	dir := initThenEdit(t,
		[2]string{"        - revive\n", ""},
		[2]string{
			"    exclusions:\n        presets:\n            - comments\n",
			"    exclusions: oops\n",
		},
	)

	got := run(t, "golangci", "-C", dir, "--off", "revive")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Equal(t, "linters.exclusions isn't laid out as in tiger's baseline, which holds "+
		"keys under it — copy linters.exclusions from tiger golangci --print\n"+
		"tiger: 1 auto rules unenforced\n", got.stdout)
	assert.Empty(t, got.stderr)
}
