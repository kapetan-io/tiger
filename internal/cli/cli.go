// Package cli is the tiger command: subcommand dispatch, run-level policy,
// and exit codes. main() is a thin wrapper over Run so the whole surface is
// testable (surface-testing).
//
// All run-level policy lives here: the registry's severity is applied to
// findings, blocking findings fail the run, and operational failures exit
// with a code distinct from findings so a partial run is never mistaken for
// a clean one.
package cli

import (
	"fmt"
	"io"
)

// Exit codes, per the blueprint: clean, findings, operational failure.
const (
	ExitClean       = 0
	ExitFindings    = 1
	ExitOperational = 2
)

const usage = `usage:
  tiger check [-C dir] [analyzer flags] [packages]
      run the registered wave-1 analyzers; exit 0 clean, 1 findings,
      2 operational failure
  tiger golangci [-C dir] [--init]
      audit the project golangci-lint config against the auto-rule baseline,
      or generate it with --init
`

// Streams carries a command's output writers — an options struct, per the
// dialect's own TS-N07.
type Streams struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Run executes one tiger invocation and returns its exit code.
func Run(args []string, streams Streams) int {
	if len(args) == 0 {
		fmt.Fprint(streams.Stderr, usage)
		return ExitOperational
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], streams)
	case "golangci":
		return runGolangci(args[1:], streams)
	case "help", "-h", "--help":
		fmt.Fprint(streams.Stdout, usage)
		return ExitClean
	default:
		fmt.Fprintf(streams.Stderr, "tiger: unknown command %q\n", args[0])
		fmt.Fprint(streams.Stderr, usage)
		return ExitOperational
	}
}
