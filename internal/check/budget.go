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

// BudgetUsage is tiger budget's entry in the command listing.
const BudgetUsage = `  tiger budget [-C dir] [--write] [packages]
      report packages over their tiger.budget.yaml rows; --write lowers
      rows to the current counts (never raises); exit 0 within budget,
      1 overrun, 2 operational failure
`

// Budget is tiger budget: the same analysis as tiger check over the
// packages, printing only the ratchet's overrun groups, exit 1 on overrun.
// --write additionally lowers tiger.budget.yaml to the current counts;
// overrun rows stay as they were (tiger never raises a budget, ADR-0011).
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
		// Overruns are judged against the file as written: a row created
		// or lowered by this run is not an overrun, an existing row that
		// stays below its count still is.
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

// setup is what every analysis-running command loads from the run
// directory before analysis: the module path, the installed config and
// layer file, and the budget file.
type setup struct {
	module  module.Path
	layers  *layerfile.File
	budgets *budget.File
}

// prepare loads and installs dir's tiger.yaml and tiger.layers.yaml, then
// loads tiger.budget.yaml; any failure is a one-line error. No file is
// required; a module path is, and only when a file exists.
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
	// The module path is optional here: without go.mod the driver fails the
	// run on its own, and with no config or budget file nothing needs it.
	path, err := module.ReadPath(dir)
	if err != nil {
		path = ""
	}
	return setup{module: path, layers: layered, budgets: budgets}, nil
}

// packageKey is the budget file's key for a package: its import path
// relative to the module, with an external test package folded into the
// package it tests, so a skipped test counts against the package it
// belongs to.
func packageKey(path module.Path, pkgPath string) string {
	pkgPath = strings.TrimSuffix(pkgPath, "_test")
	if rel, inModule := path.Relative(pkgPath); inModule {
		return rel
	}
	return pkgPath
}

// budgetCounts tallies advisory findings per budget key, keeping the
// findings themselves so an overrun can print them.
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

// add counts one advisory finding under its package key and rule code.
func (c budgetCounts) add(pkg string, entry rules.CustomRule, finding driver.Finding) {
	key := budget.Key{Package: pkg, Code: entry.RuleID}
	c.numbers[key]++
	c.findings[key] = append(c.findings[key], finding)
	c.entries[entry.RuleID] = entry
}

// printRatchet prints one TS-D06 group per overrun — the ratchet line
// positioned at the budget row, then every counted finding for that
// package and code as an ordinary blocking line — and returns the number
// of lines printed. Under budget nothing prints.
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

// ratchetMessage renders the TS-D06 line for one overrun: the package, its
// count in the rule's counted noun, and the edit — fix the excess, or a
// reviewed raise; or, with no row, fix the findings or record the count.
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
