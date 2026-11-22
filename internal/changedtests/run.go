// Package changedtests reports the test functions that existed at the merge
// base of a base ref and HEAD and that the working tree modifies or deletes.
package changedtests

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/revision"
)

const synopsis = "tiger changed-tests [-C dir] [--format text|json] <base>"

const Usage = "  " + synopsis + `
      list the test functions that existed at the merge base of <base> and
      HEAD and are modified or deleted in the working tree; flags go before
      <base>; exit 0 for any report, 2 when no report can be produced
`

func Run(args []string, streams command.Streams) int {
	flags := flag.NewFlagSet("tiger changed-tests", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	chdir := flags.String("C", ".", "run as if tiger was started in this directory")
	format := flags.String("format", "text", "report format: text or json")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			err = errors.New("usage: " + synopsis)
		}
		return failure(streams, err)
	}
	if flags.NArg() == 0 {
		return failure(streams, errors.New(
			"expected a base revision, as in: tiger changed-tests [-C dir] <base>"))
	}
	if flags.NArg() > 1 {
		return failure(streams, fmt.Errorf(
			"%q comes after the base revision; put flags before it, "+
				"as in: tiger changed-tests [-C dir] <base>",
			flags.Arg(1)))
	}
	if *format != "text" && *format != "json" {
		return failure(streams, fmt.Errorf(
			"unknown format %q; the formats are: text, json", *format))
	}
	repo, err := revision.Open(*chdir)
	if err != nil {
		return failure(streams, err)
	}
	rev, err := repo.Compare(flags.Arg(0))
	if err != nil {
		return failure(streams, err)
	}
	report, err := compareRevisions(*chdir, rev)
	if err != nil {
		return failure(streams, err)
	}
	if *format == "json" {
		data, err := report.JSON(rev)
		if err != nil {
			return failure(streams, err)
		}
		if _, err := streams.Stdout.Write(data); err != nil {
			return failure(streams, err)
		}
		return command.ExitClean
	}
	fmt.Fprint(streams.Stdout, report.Text(rev))
	return command.ExitClean
}

func compareRevisions(dir string, rev revision.Comparison) (Report, error) {
	head, err := readTests(dir)
	if err != nil {
		return Report{}, fmt.Errorf("cannot read the tests at head (%s): %w",
			revision.Short(rev.Head), err)
	}
	base, err := readBase(rev)
	if err != nil {
		return Report{}, err
	}
	return compare(Sides{Base: base, Head: head}), nil
}

func readBase(rev revision.Comparison) (tests Tests, err error) {
	tree, err := rev.Extract()
	if err != nil {
		return tests, err
	}
	defer func() { err = errors.Join(err, tree.Close()) }()
	if !tree.Present {
		return tests, nil
	}
	tests, err = readTests(tree.Dir)
	if err != nil {
		return tests, fmt.Errorf("cannot read the tests at base (%s): %w",
			revision.Short(rev.MergeBase), err)
	}
	return tests, nil
}

func failure(streams command.Streams, err error) int {
	line := strings.Join(strings.Fields(err.Error()), " ")
	fmt.Fprintf(streams.Stderr, "tiger changed-tests: %s\n", line)
	return command.ExitOperational
}
