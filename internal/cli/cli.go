// Package cli is the tiger command's name lookup and command listing. Each
// subcommand's function lives in the package that owns its concept, except
// golangci, pattern and pin, which move out in later pieces; main() is a thin
// wrapper over Run so the whole surface is testable (surface-testing).
package cli

import (
	"fmt"

	"github.com/kapetan-io/tiger/internal/changed"
	"github.com/kapetan-io/tiger/internal/check"
	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/pattern"
)

// Transitional aliases for the golangci, pattern and pin code and the tests
// that still enter through cli.Run; they go when the last of those moves.
type Streams = command.Streams

const (
	ExitClean       = command.ExitClean
	ExitFindings    = command.ExitFindings
	ExitOperational = command.ExitOperational
)

const usage = `usage:
` + check.Usage + check.BudgetUsage + `  tiger golangci [-C dir] [--init | --print]
      audit the project golangci-lint config against the auto-rule baseline,
      generate it with --init, or write it to stdout with --print
` + pattern.Usage + changed.Usage + `  tiger pin [-C dir] [--dry-run] <Name> [<Name>...] [packages]
      freeze each named function's computed facts into //tiger: pins
      written at their targets; exit 0 written or already satisfied,
      1 refusals, 2 could not proceed
run "tiger <command> -h" for its flags
`

// Run executes one tiger invocation and returns its exit code.
func Run(args []string, streams Streams) int {
	if len(args) == 0 {
		fmt.Fprint(streams.Stderr, usage)
		return ExitOperational
	}
	switch args[0] {
	case "check":
		return check.Run(args[1:], streams)
	case "budget":
		return check.Budget(args[1:], streams)
	case "golangci":
		return runGolangci(args[1:], streams)
	case "pattern":
		return pattern.Run(args[1:], streams)
	case "changed":
		return changed.Run(args[1:], streams)
	case "pin":
		return runPin(args[1:], streams)
	case "help", "-h", "--help":
		fmt.Fprint(streams.Stdout, usage)
		return ExitClean
	default:
		fmt.Fprintf(streams.Stderr, "tiger: unknown command %q\n", args[0])
		fmt.Fprint(streams.Stderr, usage)
		return ExitOperational
	}
}
