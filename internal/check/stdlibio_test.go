package check_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kapetan-io/tiger/internal/command"
)

// TestCheckDecidesIOFromStdlibList covers TS-M10's IO list through tiger
// check: pure calls in os, net and database/sql stay silent in a loop, and
// each listed IO call in a loop is reported.
//
// Goal: Pure in the fixture has no TS-M10 finding, and IO has exactly one
// TS-M10 finding per IO call, on lines 34 through 39.
func TestCheckDecidesIOFromStdlibList(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/stdlibio", "./...")
	assert.Equal(t, command.ExitFindings, got.code, got.stderr)

	var lines []string
	for _, line := range strings.Split(got.stdout, "\n") {
		if !strings.Contains(line, "TS-M10") {
			continue
		}
		file, _, _ := strings.Cut(line, ": TS-M10")
		lines = append(lines, file[:strings.LastIndex(file, ":")])
	}
	assert.Equal(t, []string{
		"stdlibio.go:34",
		"stdlibio.go:35",
		"stdlibio.go:36",
		"stdlibio.go:37",
		"stdlibio.go:38",
		"stdlibio.go:39",
	}, lines, got.stdout)
}
