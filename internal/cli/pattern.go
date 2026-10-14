package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/kapetan-io/tiger/internal/pattern"
	"github.com/kapetan-io/tiger/patterns"
)

// runPattern is tiger pattern: print a pattern from the collection embedded
// in the binary, list the patterns behind a rule, or list every pattern.
// Flags may come before or after the pattern name.
func runPattern(args []string, streams Streams) int {
	flags := flag.NewFlagSet("tiger pattern", flag.ContinueOnError)
	flags.SetOutput(streams.Stderr)
	rule := flags.String("rule", "", "list the patterns behind this rule code")
	checklist := flags.Bool("checklist", false, "print only the reviewer checklist")
	if err := flags.Parse(args); err != nil {
		return ExitOperational
	}
	names := []string{}
	if flags.NArg() > 0 {
		names = append(names, flags.Arg(0))
		if err := flags.Parse(flags.Args()[1:]); err != nil {
			return ExitOperational
		}
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(streams.Stderr, "tiger pattern: expected at most one pattern name\n")
		return ExitOperational
	}
	all, err := pattern.Load(patterns.Collection)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger pattern: %v\n", err)
		return ExitOperational
	}
	switch {
	case *rule != "" && len(names) > 0:
		fmt.Fprintf(streams.Stderr, "tiger pattern: --rule and a pattern name cannot be combined\n")
		return ExitOperational
	case *rule != "":
		show := printSummary
		if *checklist {
			show = printChecklist
		}
		return printBehind(all, *rule, show, streams)
	case len(names) == 0 && *checklist:
		fmt.Fprintf(streams.Stderr, "tiger pattern: --checklist needs a pattern name or --rule\n")
		return ExitOperational
	case len(names) == 0:
		printIndex(all, streams)
		return ExitClean
	}
	found, known := pattern.Find(all, names[0])
	if !known {
		fmt.Fprintf(streams.Stderr, "tiger pattern: unknown pattern %q\n", names[0])
		printIndex(all, Streams{Stdout: streams.Stderr, Stderr: streams.Stderr})
		return ExitOperational
	}
	if *checklist {
		printChecklist(found, streams)
		return ExitClean
	}
	printPattern(found, streams)
	return ExitClean
}

// printIndex lists every pattern with its summary.
func printIndex(all []pattern.Pattern, streams Streams) {
	for _, each := range all {
		printSummary(each, streams)
	}
}

// printBehind shows each pattern that backs one rule code, as a summary
// line or as its checklist. A rule with no pattern is an error so a
// mistyped code does not look like an empty answer.
func printBehind(
	all []pattern.Pattern,
	code string,
	show func(pattern.Pattern, Streams),
	streams Streams,
) int {
	found := pattern.Behind(all, code)
	if len(found) == 0 {
		fmt.Fprintf(streams.Stderr, "tiger pattern: no pattern backs %s\n", code)
		return ExitOperational
	}
	for _, each := range found {
		show(each, streams)
	}
	return ExitClean
}

func printSummary(each pattern.Pattern, streams Streams) {
	summary := strings.Join(strings.Fields(each.Summary), " ")
	fmt.Fprintf(streams.Stdout, "%s\t%s\n", each.Name, summary)
}

func printChecklist(each pattern.Pattern, streams Streams) {
	fmt.Fprintf(streams.Stdout, "%s\n", each.Name)
	for _, item := range each.Checklist {
		fmt.Fprintf(streams.Stdout, "- %s\n", item)
	}
}

// printPattern prints the card, then the code, then the tests, each file
// under a header naming its path so an agent can copy the shape.
func printPattern(each pattern.Pattern, streams Streams) {
	fmt.Fprintf(streams.Stdout, "# %s\n\n%s\n\nRules: %s\n\nGap: %s\n\nBug: %s\n\nChecklist:\n",
		each.Name, each.Summary, strings.Join(each.Rules, ", "), each.Gap, each.Bug)
	for _, item := range each.Checklist {
		fmt.Fprintf(streams.Stdout, "- %s\n", item)
	}
	for _, tests := range []bool{false, true} {
		heading := "Code"
		if tests {
			heading = "Tests"
		}
		for _, file := range each.Files {
			if file.Test != tests {
				continue
			}
			fmt.Fprintf(streams.Stdout, "\n## %s: %s/%s\n\n%s",
				heading, each.Name, file.Path, file.Source)
		}
	}
}
