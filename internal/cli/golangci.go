package cli

import (
	"flag"
	"fmt"

	"github.com/kapetan-io/tiger/internal/golangci"
)

// runGolangci is tiger golangci: audit the project's golangci-lint config
// against the registry's auto-rule baseline, or generate it with --init.
// Exit codes mirror tiger check: 0 conforming, 1 non-conforming,
// 2 operational — and --init refusal is operational, because exit 1 is
// reserved for a verification verdict.
func runGolangci(args []string, streams Streams) int {
	flags := flag.NewFlagSet("tiger golangci", flag.ContinueOnError)
	flags.SetOutput(streams.Stderr)
	chdir := flags.String("C", ".", "run as if tiger was started in this directory")
	initConfig := flags.Bool("init", false,
		"write the baseline config for a project that has none; refuses to touch an existing one")
	if err := flags.Parse(args); err != nil {
		return ExitOperational
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(streams.Stderr, "tiger golangci: unexpected argument %q\n", flags.Arg(0))
		return ExitOperational
	}

	if *initConfig {
		path, err := golangci.Init(*chdir)
		if err != nil {
			fmt.Fprintf(streams.Stderr, "tiger golangci: %v\n", err)
			return ExitOperational
		}
		fmt.Fprintf(streams.Stdout, "wrote %s\n", path)
		return ExitClean
	}

	findings, err := golangci.Verify(*chdir)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger golangci: %v\n", err)
		return ExitOperational
	}
	for _, finding := range findings {
		fmt.Fprintf(streams.Stdout, "%s\n", finding.Message)
	}
	if len(findings) > 0 {
		fmt.Fprintf(streams.Stdout, "tiger: %d auto rules unenforced\n", len(findings))
		return ExitFindings
	}
	return ExitClean
}
