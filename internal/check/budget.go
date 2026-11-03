package check

import (
	"flag"
	"fmt"
	"go/token"
	"maps"
	"slices"
	"strings"

	"github.com/kapetan-io/tiger/internal/budget"
	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/config"
	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/layerfile"
	"github.com/kapetan-io/tiger/internal/module"
	"github.com/kapetan-io/tiger/internal/rules"
)

const BudgetUsage = `  tiger budget [-C dir] [--write] [packages]
      report packages over their tiger.budget.yaml rows; --write lowers
      rows to the current counts (never raises); exit 0 within budget,
      1 overrun, 2 operational failure
`

func Budget(args []string, streams command.Streams) int {
	flags := flag.NewFlagSet("tiger budget", flag.ContinueOnError)
	flags.SetOutput(streams.Stderr)
	flags.Usage = func() { command.PrintUsage(flags, BudgetUsage) }
	chdir := flags.String("C", ".", "run as if tiger was started in this directory")
	write := flags.Bool("write", false,
		"lower every analyzed row to its current count, creating missing rows and "+
			"deleting rows at 0; never raises a row")
	if err := flags.Parse(args); err != nil {
		return command.ExitOperational
	}
	patterns := flags.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	setup, err := prepare(*chdir)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger budget: %v\n", err)
		return command.ExitOperational
	}
	report, err := driver.Run(*chdir, patterns, rules.Analyzers(), rules.Finishers())
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger budget: %v\n", err)
		return command.ExitOperational
	}
	sorted, err := partition(setup.module, report.Findings)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger budget: %v\n", err)
		return command.ExitOperational
	}
	budgets := setup.budgets
	if *write {
		analyzed := []string{}
		for _, pkgPath := range report.Packages {
			analyzed = append(analyzed, packageKey(setup.module, pkgPath))
		}
		budgets = setup.budgets.Lower(sorted.counts.numbers, budget.Run{
			Packages: analyzed,
			Codes:    slices.Sorted(maps.Keys(rules.CountedCodes())),
		})
		if err := budgets.Write(*chdir); err != nil {
			fmt.Fprintf(streams.Stderr, "tiger budget: %v\n", err)
			return command.ExitOperational
		}
	}
	if printRatchet(streams, budgets, sorted) > 0 {
		return command.ExitFindings
	}
	return command.ExitClean
}

type setup struct {
	module  module.Path
	layers  *layerfile.File
	budgets *budget.File
}

func prepare(dir string) (setup, error) {
	loaded, err := config.Load(dir, rules.Analyzers())
	if err != nil {
		return setup{}, err
	}
	config.Install(loaded)
	layered, err := layerfile.Load(dir)
	if err != nil {
		return setup{}, err
	}
	layerfile.Install(layered)
	budgets, err := budget.Load(dir, rules.CountedCodes())
	if err != nil {
		return setup{}, err
	}
	modulePath, err := module.ReadPath(dir)
	if err != nil {
		modulePath = ""
	}
	return setup{module: modulePath, layers: layered, budgets: budgets}, nil
}

func packageKey(modulePath module.Path, pkgPath string) string {
	pkgPath = strings.TrimSuffix(pkgPath, "_test")
	if rel, inModule := modulePath.Relative(pkgPath); inModule {
		return rel
	}
	return pkgPath
}

type budgetCounts struct {
	numbers  budget.Counts
	findings map[budget.Key][]driver.Finding
	entries  map[string]rules.CustomRule
}

func newBudgetCounts() budgetCounts {
	return budgetCounts{
		numbers:  budget.Counts{},
		findings: map[budget.Key][]driver.Finding{},
		entries:  map[string]rules.CustomRule{},
	}
}

func (c budgetCounts) add(pkg string, entry rules.CustomRule, finding driver.Finding) {
	key := budget.Key{Package: pkg, Code: entry.RuleID}
	c.numbers[key]++
	c.findings[key] = append(c.findings[key], finding)
	c.entries[entry.RuleID] = entry
}

func printRatchet(streams command.Streams, budgets *budget.File, sorted partitioned) int {
	printed := 0
	for _, overrun := range budgets.Compare(sorted.counts.numbers) {
		position := token.Position{Filename: budget.FileName, Line: overrun.Line}
		fmt.Fprintf(streams.Stdout, "%s: %s\n", position,
			ratchetMessage(overrun, sorted.counts.entries[overrun.Key.Code]))
		printed++
		for _, finding := range sorted.counts.findings[overrun.Key] {
			fmt.Fprintf(streams.Stdout, "%s: %s\n", finding.Position, finding.Message)
			printed++
		}
	}
	return printed
}

func ratchetMessage(overrun budget.Overrun, entry rules.CustomRule) string {
	noun := entry.CountedNoun(overrun.Count)
	if overrun.NoRow {
		them := "them"
		if overrun.Count == 1 {
			them = "it"
		}
		return fmt.Sprintf("TS-D06: %s has %d %s and %s has no row for it — fix %s, or run "+
			"tiger budget --write to record the current count",
			overrun.Key.Package, overrun.Count, noun, budget.FileName, them)
	}
	return fmt.Sprintf("TS-D06: %s has %d %s but %s allows %d — fix %d of them, or raise the "+
		"number in a reviewed edit",
		overrun.Key.Package, overrun.Count, noun, budget.FileName, overrun.Budget,
		overrun.Count-overrun.Budget)
}
