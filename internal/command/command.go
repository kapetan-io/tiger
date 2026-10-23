// Package command is a subcommand's contract with the shell: the two output
// streams it writes, the exit codes it returns, and the shape of its -h
// output. cli.Run looks a subcommand up and hands it the arguments after its
// name; the package that owns the command holds the function behind it.
package command

import (
	"flag"
	"fmt"
	"io"
)

// Streams carries a command's output writers, an options struct per tiger's
// own TS-N07.
type Streams struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Exit codes: clean, findings, operational failure.
const (
	ExitClean       = 0
	ExitFindings    = 1
	ExitOperational = 2
)

// PrintUsage writes "usage:", the command's entry in the command listing,
// then the flag defaults, to the flag set's output.
func PrintUsage(flags *flag.FlagSet, usage string) {
	fmt.Fprint(flags.Output(), "usage:\n"+usage)
	flags.PrintDefaults()
}
