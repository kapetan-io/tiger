package check

import (
	"go/token"

	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/layerfile"
)

func layerCoverage(setup setup, blocking []driver.Finding) []driver.Finding {
	if setup.layers.Empty() {
		return nil
	}
	coverage := setup.layers.Coverage()
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
