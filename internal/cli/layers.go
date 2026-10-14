package cli

import (
	"go/token"

	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/layerfile"
)

// layerCoverage returns the TS-X03 findings that need the module's package
// directories: an entry that matches no package directory, and a package
// with no entry that the layers analyzer did not report. This is not a
// finish step, because a finish step sees only the packages the run loaded,
// and these checks also need the directories it skipped: those outside the
// patterns passed, built only for another platform, or holding only
// generated files, whose findings the driver drops.
func layerCoverage(setup setup, blocking []driver.Finding) []driver.Finding {
	if setup.layers.Empty() {
		return nil
	}
	coverage := setup.layers.Coverage()
	// The analyzer's only TS-X03 finding for a package with no entry is the
	// missing entry, so any TS-X03 finding from such a package means it was
	// reported already. Unlike the budget key, a `_test` suffix is not
	// stripped: the analyzer never reports from `_test.go` files, so a path
	// ending in `_test` is a real directory such as ./e2e_test.
	reported := map[string]bool{}
	for _, finding := range blocking {
		rel, inModule := setup.module.Relative(finding.Package)
		if finding.Category == layerfile.RuleID && inModule {
			reported[rel] = true
		}
	}
	findings := []driver.Finding{}
	for _, pkg := range coverage.Unlisted {
		if reported[pkg.Dir] {
			continue
		}
		findings = append(findings, driver.Finding{
			Position: token.Position{Filename: pkg.File, Line: 1, Column: 1},
			Category: layerfile.RuleID,
			Message:  layerfile.MissingEntryMessage(pkg.Dir),
		})
	}
	for _, stale := range coverage.Stale {
		findings = append(findings, driver.Finding{
			Position: token.Position{Filename: layerfile.FileName, Line: stale.Line, Column: 1},
			Category: layerfile.RuleID,
			Message:  layerfile.StaleMessage(stale.Key),
		})
	}
	return findings
}
