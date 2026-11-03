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

type Streams struct {
	Stdout io.Writer
	Stderr io.Writer
}

const (
	ExitClean       = 0
	ExitFindings    = 1
	ExitOperational = 2
)

func PrintUsage(flags *flag.FlagSet, usage string) {
	fmt.Fprint(flags.Output(), "usage:\n"+usage)
	flags.PrintDefaults()
}
