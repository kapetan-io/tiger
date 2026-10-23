package golangci_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
