package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPatternHelpNamesTheCommand reads the usage text.
//
// Goal: tiger help documents tiger pattern with its two flags.
func TestPatternHelpNamesTheCommand(t *testing.T) {
	got := run(t, "help")
	assert.Contains(t, got.stdout, "tiger pattern [name] [--checklist] [--rule code]")
}
