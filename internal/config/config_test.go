package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/config"
)

func TestLoadMalformedYAMLIsOneLine(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n"), 0o600))
	// Multi-line parser errors break the one-line finding format (ADR-0009).
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte("version: 1\n\tbad: [\n  - x\n"), 0o600))

	_, err := config.Load(dir, nil)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "\n")
}

func TestDirectoriesRejectsCommandLineValue(t *testing.T) {
	// A flag value has no room for the reason each directory needs.
	err := config.Directories{Name: "comments"}.Set("internal/a")

	require.ErrorContains(t, err, "is set only in "+config.FileName)
}

func TestInstallNilResetsConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte("version: 1\n"), 0o600))
	file, err := config.Load(dir, nil)
	require.NoError(t, err)
	require.NotNil(t, file)

	config.Install(file)
	assert.True(t, config.Installed())

	// Every run installs its own config, so an absent file must clear the last one.
	config.Install(nil)
	assert.False(t, config.Installed())
}
