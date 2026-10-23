// Package cli is the tiger command's name lookup and command listing. main()
// is a thin wrapper over Run so the whole surface is testable
// (surface-testing).
package cli

import (
	"fmt"

	"github.com/kapetan-io/tiger/internal/changed"
	"github.com/kapetan-io/tiger/internal/check"
	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/golangci"
	"github.com/kapetan-io/tiger/internal/pattern"
	"github.com/kapetan-io/tiger/internal/pin"
)

const usage = `usage:
` + check.Usage + check.BudgetUsage + golangci.Usage + pattern.Usage +
	changed.Usage + pin.Usage + `run "tiger <command> -h" for its flags
`

// Run executes one tiger invocation and returns its exit code.
func Run(args []string, streams command.Streams) int {
	if len(args) == 0 {
		fmt.Fprint(streams.Stderr, usage)
		return command.ExitOperational
	}
	switch args[0] {
	case "check":
		return check.Run(args[1:], streams)
	case "budget":
		return check.Budget(args[1:], streams)
	case "golangci":
		return golangci.Run(args[1:], streams)
	case "pattern":
		return pattern.Run(args[1:], streams)
	case "changed":
		return changed.Run(args[1:], streams)
	case "pin":
		return pin.Run(args[1:], streams)
	case "help", "-h", "--help":
		fmt.Fprint(streams.Stdout, usage)
		return command.ExitClean
	default:
		fmt.Fprintf(streams.Stderr, "tiger: unknown command %q\n", args[0])
		fmt.Fprint(streams.Stderr, usage)
		return command.ExitOperational
	}
}
