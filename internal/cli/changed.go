package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kapetan-io/tiger/internal/callinventory"
	"github.com/kapetan-io/tiger/internal/changed"
	"github.com/kapetan-io/tiger/internal/revision"
)

// runChanged is tiger changed: compare the calls each function makes at the
// merge base of a base ref and HEAD against the calls it makes in the working
// tree, and print the report. A report exits 0 whatever it holds. When none
// can be produced it prints one line to stderr and exits 2, with nothing on
// stdout, so a failed run is never mistaken for an empty report.
func runChanged(args []string, streams Streams) int {
	flags := flag.NewFlagSet("tiger changed", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	chdir := flags.String("C", ".", "run as if tiger was started in this directory")
	format := flags.String("format", "text", "report format: text or json")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			err = errors.New("usage: tiger changed [-C dir] <base>")
		}
		return changedFailure(streams, err)
	}
	if flags.NArg() == 0 {
		return changedFailure(streams, errors.New(
			"expected a base revision, as in: tiger changed [-C dir] <base>"))
	}
	if flags.NArg() > 1 {
		return changedFailure(streams, fmt.Errorf(
			"%q comes after the base revision; put flags before it, "+
				"as in: tiger changed [-C dir] <base>",
			flags.Arg(1)))
	}
	if *format != "text" && *format != "json" {
		return changedFailure(streams, fmt.Errorf(
			"unknown format %q; the formats are: text, json", *format))
	}
	repo, err := revision.Open(*chdir)
	if err != nil {
		return changedFailure(streams, err)
	}
	rev, err := repo.Compare(flags.Arg(0))
	if err != nil {
		return changedFailure(streams, err)
	}
	report, err := compareRevisions(*chdir, rev)
	if err != nil {
		return changedFailure(streams, err)
	}
	if *format == "json" {
		data, err := report.JSON(rev)
		if err != nil {
			return changedFailure(streams, err)
		}
		streams.Stdout.Write(data)
		return ExitClean
	}
	fmt.Fprint(streams.Stdout, report.Text(rev))
	return ExitClean
}

// compareRevisions loads the head tree and the base tree and compares them.
func compareRevisions(dir string, rev revision.Comparison) (report changed.Report, err error) {
	headLabel := "head (" + revision.Short(rev.Head) + ")"
	head, err := callinventory.Load(dir)
	if err != nil {
		return changed.Report{}, fmt.Errorf("cannot load the Go packages at %s: %w", headLabel, err)
	}
	if len(head.Packages) == 0 {
		return changed.Report{}, noPackages(head, headLabel)
	}
	tree, err := rev.Extract()
	if err != nil {
		return changed.Report{}, err
	}
	defer func() { err = errors.Join(err, tree.Close()) }()
	var base callinventory.Inventory
	if tree.Present {
		if base, err = callinventory.Load(tree.Dir); err != nil {
			return changed.Report{}, fmt.Errorf("cannot load the Go packages at base (%s): %w",
				revision.Short(rev.MergeBase), err)
		}
	}
	return changed.Compare(changed.Sides{Base: base, Head: head}), nil
}

func noPackages(inv callinventory.Inventory, label string) error {
	if inv.LoadError != "" {
		return fmt.Errorf("no Go package loads at %s: %s", label, inv.LoadError)
	}
	return fmt.Errorf("no Go packages found at %s", label)
}

// changedFailure prints the one line a failed run owes the reader.
func changedFailure(streams Streams, err error) int {
	line := strings.Join(strings.Fields(err.Error()), " ")
	fmt.Fprintf(streams.Stderr, "tiger changed: %s\n", line)
	return ExitOperational
}
