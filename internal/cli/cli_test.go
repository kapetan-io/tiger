package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// outcome is one CLI invocation's observable result.
type outcome struct {
	code   int
	stdout string
	stderr string
}

// run invokes the CLI surface once.
func run(t *testing.T, args ...string) outcome {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := cli.Run(args, cli.Streams{Stdout: stdout, Stderr: stderr})
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// TestUnknownCommandExitsTwo covers subcommand dispatch.
//
// Goal: an unknown subcommand is an operational error carrying usage, and
// help exits clean.
func TestUnknownCommandExitsTwo(t *testing.T) {
	unknown := run(t, "frobnicate")
	assert.Equal(t, cli.ExitOperational, unknown.code)
	assert.Contains(t, unknown.stderr, "unknown command")
	require.Contains(t, unknown.stderr, "usage:")

	helped := run(t, "help")
	assert.Equal(t, cli.ExitClean, helped.code)
	assert.Contains(t, helped.stdout, "tiger check")

	bare := run(t)
	assert.Equal(t, cli.ExitOperational, bare.code)
	assert.Contains(t, bare.stderr, "usage:")
}

// TestChangedIsListedAndDispatched covers the name lookup for a command whose
// code lives in its own package.
//
// Goal: the command listing carries tiger changed's entry, and the name
// reaches that command.
func TestChangedIsListedAndDispatched(t *testing.T) {
	bare := run(t)
	assert.Contains(t, bare.stderr, "tiger changed [-C dir] [--format text|markdown|json] <base>")

	got := run(t, "changed")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Equal(t, "tiger changed: expected a base revision, "+
		"as in: tiger changed [-C dir] <base>\n", got.stderr)
}
