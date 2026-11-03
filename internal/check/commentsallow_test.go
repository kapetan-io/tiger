package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
)

func commentsTree(t *testing.T, yaml string) string {
	t.Helper()
	dir := copyFixture(t, "commentsallow")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tiger.yaml"), []byte(yaml), 0o644))
	return dir
}

func TestCommentsAllowExemptsTheSubtree(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "commentsallow"), "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "fresh/fresh.go:3:1: TS-L14: this comment is below the package line of a "+
		"production file — tiger expects the code, its tests and //tiger: directives to carry "+
		"meaning here, not prose — move what it says into a test, or delete it\n"+
		"tiger: 1 blocking\n",
		got.stdout)
}

func TestCommentsAllowDotExemptsTheModule(t *testing.T) {
	dir := commentsTree(t, allowing("."))
	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

func TestCommentsAllowFindingNeverNamesTheSetting(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "commentsallow"), "./...")
	assert.NotContains(t, got.stdout, "comments.allow")
}

func TestCommentsAllowRejectsInvalidEntries(t *testing.T) {
	for _, test := range []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "EmptyReason",
			yaml:    "version: 1\ncomments:\n  allow:\n    - value: legacy\n      reason: \"\"\n",
			wantErr: rejected + "legacy: the entry has no reason",
		},
		{
			name: "Packages",
			yaml: "version: 1\ncomments:\n  allow:\n    - value: legacy\n      reason: r\n" +
				"      packages: [legacy]\n",
			wantErr: rejected + "legacy: the entry has packages:",
		},
		{
			name:    "MissingDirectory",
			yaml:    allowing("gone"),
			wantErr: rejected + "gone: no directory has this path",
		},
		{
			name:    "FileNotDirectory",
			yaml:    allowing("go.mod"),
			wantErr: rejected + "go.mod: no directory has this path",
		},
		{
			name:    "NestedModule",
			yaml:    allowing("nested/inside"),
			wantErr: rejected + "nested/inside: the directory is inside the nested module",
		},
		{
			name:    "Ellipsis",
			yaml:    allowing("legacy/..."),
			wantErr: rejected + "legacy/...: the value is not a module-relative directory",
		},
		{
			name:    "AllEllipsis",
			yaml:    allowing("..."),
			wantErr: rejected + "...: the value is not a module-relative directory",
		},
		{
			name:    "Parent",
			yaml:    allowing("../legacy"),
			wantErr: rejected + "../legacy: the value is not a module-relative directory",
		},
		{
			name:    "Absolute",
			yaml:    allowing("/tmp"),
			wantErr: rejected + "/tmp: the value is not a module-relative directory",
		},
		{
			name:    "TrailingSlash",
			yaml:    allowing("legacy/"),
			wantErr: rejected + "legacy/: the value is not a module-relative directory",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := commentsTree(t, test.yaml)
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested", "inside"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "nested", "go.mod"),
				[]byte("module fixture.example/nested\n\ngo 1.26\n"), 0o644))
			got := run(t, "check", "-C", dir, "./...")
			assert.Equal(t, command.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			require.Len(t, splitLines(got.stderr), 1)
			assert.Contains(t, got.stderr, test.wantErr)
		})
	}
}

func TestCommentsAllowRefusesTheCommandLine(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "commentsallow"),
		"-comments.allow=legacy", "./...")
	assert.Equal(t, command.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, `invalid value "legacy" for flag -comments.allow: `+
		"comments.allow is set only in tiger.yaml, each directory with a reason")
}

const rejected = "tiger.yaml: comments: allow: "

func allowing(value string) string {
	return "version: 1\ncomments:\n  allow:\n    - value: " + value + "\n      reason: r\n"
}
