package golangci

import (
	"flag"
	"fmt"
	"strings"

	"github.com/kapetan-io/tiger/internal/command"
)

const Usage = `  tiger golangci [-C dir] [--off linter,...] [--init | --print]
      audit the project golangci-lint config against the auto-rule baseline,
      not reporting the linters --off names as missing; generate the baseline
      with --init, or write it to stdout with --print
`

func Run(args []string, streams command.Streams) int {
	flags := flag.NewFlagSet("tiger golangci", flag.ContinueOnError)
	flags.SetOutput(streams.Stderr)
	flags.Usage = func() { command.PrintUsage(flags, Usage) }
	chdir := flags.String("C", ".", "run as if tiger was started in this directory")
	initConfig := flags.Bool("init", false,
		"write the baseline config for a project that has none; refuses to touch an existing one")
	printConfig := flags.Bool("print", false,
		"write the baseline config to stdout, whether or not a config exists")
	off := []string{}
	flags.Func("off", "comma-separated linters left off on purpose; "+
		"the audit does not report them missing from the enable lists",
		func(value string) error {
			for name := range strings.SplitSeq(value, ",") {
				off = append(off, strings.TrimSpace(name))
			}
			return nil
		})
	if err := flags.Parse(args); err != nil {
		return command.ExitOperational
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(streams.Stderr, "tiger golangci: unexpected argument %q\n", flags.Arg(0))
		return command.ExitOperational
	}
	if *initConfig && *printConfig {
		fmt.Fprint(streams.Stderr, "tiger golangci: --init and --print are exclusive — "+
			"use --init to write a file or --print to write to stdout\n")
		return command.ExitOperational
	}

	if len(off) > 0 {
		if *initConfig || *printConfig {
			fmt.Fprint(streams.Stderr, "tiger golangci: --off applies only to the audit — "+
				"drop --off, or drop --init and --print\n")
			return command.ExitOperational
		}
	}

	if *printConfig {
		generated, err := Print(*chdir)
		if err != nil {
			fmt.Fprintf(streams.Stderr, "tiger golangci: %v\n", err)
			return command.ExitOperational
		}
		if _, err := streams.Stdout.Write(generated); err != nil {
			fmt.Fprintf(streams.Stderr, "tiger golangci: %v\n", err)
			return command.ExitOperational
		}
		return command.ExitClean
	}

	if *initConfig {
		path, err := Init(*chdir)
		if err != nil {
			fmt.Fprintf(streams.Stderr, "tiger golangci: %v\n", err)
			return command.ExitOperational
		}
		fmt.Fprintf(streams.Stdout, "wrote %s\n", path)
		return command.ExitClean
	}

	findings, err := Verify(*chdir, off...)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger golangci: %v\n", err)
		return command.ExitOperational
	}
	for _, finding := range findings {
		fmt.Fprintf(streams.Stdout, "%s\n", finding.Message)
	}
	if len(findings) > 0 {
		fmt.Fprintf(streams.Stdout, "tiger: %d auto rules unenforced\n", len(findings))
		return command.ExitFindings
	}
	return command.ExitClean
}
