package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/rules"
)

// runCheck is tiger check: load the named packages, run every registered
// analyzer, print findings ordered by position, and apply the registry's
// severity to the exit code.
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

	findings, err := driver.Check(*chdir, patterns, analyzers)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger check: %v\n", err)
		return ExitOperational
	}
	blocking, advisory := 0, 0
	for _, finding := range findings {
		entry, known := rules.ByCategory(finding.Category)
		if !known {
			fmt.Fprintf(
				streams.Stderr,
				"tiger check: analyzer emitted unregistered category %q at %s\n",
				finding.Category,
				finding.Position,
			)
			return ExitOperational
		}
		switch entry.Severity {
		case rules.SeverityBlocking:
			blocking++
			fmt.Fprintf(streams.Stdout, "%s: %s\n", finding.Position, finding.Message)
		case rules.SeverityAdvisory:
			advisory++
			fmt.Fprintf(
				streams.Stdout,
				"%s: %s\n",
				finding.Position,
				markAdvisory(finding, entry.RuleID),
			)
		case rules.SeverityReported:
			// Reported findings are collected like every other finding but
			// never counted and never printed unless the caller asked to
			// see them: a fixture with only unpinned facts and no
			// violations must exit 0 and, without --show-facts, produce
			// byte-identical output to a wave-1 run over the same tree.
			if *showFacts {
				fmt.Fprintf(streams.Stdout, "%s: %s\n", finding.Position, finding.Message)
			}
		default:
			assert.Unreachable("severity outside the registry's closed set")
		}
	}
	if blocking+advisory > 0 {
		fmt.Fprintf(streams.Stdout, "tiger: %d blocking, %d advisory\n", blocking, advisory)
	}
	if blocking > 0 {
		return ExitFindings
	}
	return ExitClean
}

// markAdvisory rewrites a finding's leading rule ID to name its severity, so
// an advisory line is unmistakable without changing what fired or the
// compliant form: "TS-L09: ..." becomes "TS-L09 [advisory]: ...".
func markAdvisory(finding driver.Finding, ruleID string) string {
	rest, found := strings.CutPrefix(finding.Message, ruleID+":")
	if !found {
		return finding.Message + " [advisory]"
	}
	return ruleID + " [advisory]:" + rest
}
