package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/config"
	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/rules"
)

// runCheck is tiger check: install the directory's config, load the named
// packages, run every registered analyzer and finish step, print blocking
// findings ordered by position, compare counted findings to
// tiger.budget.yaml, and apply the registry's severity to the exit code.
func runCheck(args []string, streams Streams) int {
	flags := flag.NewFlagSet("tiger check", flag.ContinueOnError)
	flags.SetOutput(streams.Stderr)
	chdir := flags.String("C", ".", "run as if tiger was started in this directory")
	showFacts := flags.Bool(
		"show-facts",
		false,
		"print computed facts (effect sets, frames, synthesized variants) in pin syntax",
	)
	analyzers := rules.Analyzers()
	for _, registered := range analyzers {
		prefix := registered.Name + "."
		registered.Flags.VisitAll(func(each *flag.Flag) {
			flags.Var(each.Value, prefix+each.Name, each.Usage)
		})
	}
	if err := flags.Parse(args); err != nil {
		return ExitOperational
	}
	patterns := flags.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	setup, err := prepare(*chdir)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger check: %v\n", err)
		return ExitOperational
	}
	if config.Installed() {
		if set := explicitAnalyzerFlag(flags); set != "" {
			fmt.Fprintf(streams.Stderr, "tiger check: -%s is set on the command line but "+
				"%s is the reviewed source — move the value into %s\n",
				set, config.FileName, config.FileName)
			return ExitOperational
		}
	}

	report, err := driver.Run(*chdir, patterns, analyzers, rules.Finishers())
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger check: %v\n", err)
		return ExitOperational
	}
	sorted, err := partition(setup.module, report.Findings)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger check: %v\n", err)
		return ExitOperational
	}
	printed := 0
	for _, finding := range sorted.blocking {
		printed++
		fmt.Fprintf(streams.Stdout, "%s: %s\n", finding.Position, finding.Message)
	}
	if *showFacts {
		for _, finding := range sorted.facts {
			fmt.Fprintf(streams.Stdout, "%s: %s\n", finding.Position, finding.Message)
		}
	}
	printed += printRatchet(streams, setup.budgets, sorted)
	if printed > 0 {
		fmt.Fprintf(streams.Stdout, "tiger: %d blocking\n", printed)
		return ExitFindings
	}
	return ExitClean
}

// explicitAnalyzerFlag returns the name of the first analyzer flag
// (<analyzer>.<flag>) the command line set explicitly, or "".
func explicitAnalyzerFlag(flags *flag.FlagSet) string {
	set := ""
	flags.Visit(func(each *flag.Flag) {
		if set == "" && strings.Contains(each.Name, ".") {
			set = each.Name
		}
	})
	return set
}

// partitioned is one run's findings split by kind: blocking findings,
// computed facts (printed only under --show-facts, never counted), and the
// advisory ones tallied per package and rule code for the ratchet.
type partitioned struct {
	blocking []driver.Finding
	facts    []driver.Finding
	counts   budgetCounts
}

// partition resolves every finding's category through the registry — the
// facts table first, then the rules table — and splits the findings by
// kind. An unregistered category is an error: the run cannot apply a
// severity it does not know.
func partition(module config.Module, findings []driver.Finding) (partitioned, error) {
	sorted := partitioned{counts: newBudgetCounts()}
	for _, finding := range findings {
		if _, isFact := rules.ByFact(finding.Category); isFact {
			sorted.facts = append(sorted.facts, finding)
			continue
		}
		entry, known := rules.ByCategory(finding.Category)
		if !known {
			return partitioned{}, fmt.Errorf("analyzer emitted unregistered category %q at %s",
				finding.Category, finding.Position)
		}
		switch entry.Severity {
		case rules.SeverityBlocking:
			sorted.blocking = append(sorted.blocking, finding)
		case rules.SeverityAdvisory:
			sorted.counts.add(packageKey(module, finding.Package), entry, finding)
		default:
			assert.Unreachable("severity outside the registry's closed set")
		}
	}
	return sorted, nil
}
