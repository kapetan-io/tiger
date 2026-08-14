// Package golangci audits the other half of the dialect: the auto rules
// delegated to golangci-lint. Verify checks a project's configuration
// against the registry's auto-rule baseline; Generate and Init derive that
// baseline config from the registry, so a rule cannot be registered without
// being both generated and audited.
package golangci

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kapetan-io/tiger/internal/rules"
)

// ErrConfigExists reports an --init refused because the project already has
// a configuration: repairing a non-conforming config is guided by
// verification findings, never by overwriting.
var ErrConfigExists = errors.New(
	"a golangci-lint config already exists; run tiger golangci to audit it",
)

// configNames are the file names probed for a project's configuration, in
// order.
var configNames = []string{".golangci.yml", ".golangci.yaml"}

// Finding is one verification gap: the auto rule that stopped being
// enforced and the config change that restores it.
type Finding struct {
	RuleID  string
	Linter  string
	Message string
}

// baseline wraps one expected configuration fragment for comparison against
// what a project's YAML actually holds.
type baseline struct {
	want any
}

// Verify reads the golangci-lint configuration in dir and checks it against
// the registry's auto-rule baseline: every required linter enabled, every
// baseline setting present with its expected value. Extra linters and
// settings pass. The returned findings are sorted; an unreadable config or
// a missing module path is an error, not a verdict.
func Verify(dir string) ([]Finding, error) {
	document, err := readConfig(dir)
	if err != nil {
		return nil, err
	}
	module, err := modulePath(dir)
	if err != nil {
		return nil, err
	}
	findings := []Finding{}
	for _, rule := range rules.AutoRules() {
		findings = append(findings, verifyRule(document, rule, module)...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].RuleID != findings[j].RuleID {
			return findings[i].RuleID < findings[j].RuleID
		}
		if findings[i].Linter != findings[j].Linter {
			return findings[i].Linter < findings[j].Linter
		}
		return findings[i].Message < findings[j].Message
	})
	return findings, nil
}

// verifyRule checks one auto rule: its linter is enabled and each baseline
// setting holds.
func verifyRule(document map[string]any, rule rules.AutoRule, module string) []Finding {
	if !enabled(document, rule) {
		return []Finding{{
			RuleID: rule.RuleID,
			Linter: rule.Linter,
			Message: fmt.Sprintf(
				"%s: %s is missing from %s.enable — auto rule %s (%s) is not "+
					"enforced; add %s to %s.enable",
				rule.RuleID,
				rule.Linter,
				rule.Section,
				rule.RuleID,
				rule.Title,
				rule.Linter,
				rule.Section,
			),
		}}
	}
	findings := []Finding{}
	for _, setting := range rule.Settings {
		joined := strings.Join(setting.Path, ".")
		want := rules.SubstituteModule(setting.Want, module)
		got, found := lookup(document, setting.Path)
		if !found {
			findings = append(findings, Finding{
				RuleID: rule.RuleID,
				Linter: rule.Linter,
				Message: fmt.Sprintf(
					"%s: setting %s is absent — auto rule %s (%s) is not enforced; set it to %v",
					rule.RuleID, joined, rule.RuleID, rule.Title, formatWant(want)),
			})
			continue
		}
		if !(baseline{want: want}).matches(got) {
			findings = append(findings, Finding{
				RuleID: rule.RuleID,
				Linter: rule.Linter,
				Message: fmt.Sprintf(
					"%s: setting %s is %v, baseline requires %v — auto rule %s (%s) drifted; "+
						"v1 compares exactly, so a stricter value also fails",
					rule.RuleID,
					joined,
					formatWant(got),
					formatWant(want),
					rule.RuleID,
					rule.Title,
				),
			})
		}
	}
	return findings
}

// enabled reports whether the rule's linter is on: named in its section's
// enable list, or a member of golangci-lint's standard default group when
// the config keeps default: standard.
func enabled(document map[string]any, rule rules.AutoRule) bool {
	names, found := lookup(document, []string{rule.Section, "enable"})
	if found {
		if listed, ok := names.([]any); ok {
			for _, name := range listed {
				if name == rule.Linter {
					return true
				}
			}
		}
	}
	if rule.Section != rules.SectionLinters || !rules.StandardLinter(rule.Linter) {
		return false
	}
	group, found := lookup(document, []string{rules.SectionLinters, "default"})
	return found && group == "standard"
}

// lookup walks a parsed YAML document by key path.
func lookup(document map[string]any, path []string) (any, bool) {
	var current any = document
	for _, key := range path {
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapped[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// matches compares a project's YAML fragment against the baseline. Scalars
// and scalar lists must match exactly; a list of maps must contain every
// baseline element (extra elements pass); a map must hold every baseline
// key (extra keys pass).
func (b baseline) matches(got any) bool {
	switch want := b.want.(type) {
	case []any:
		listed, ok := got.([]any)
		if !ok {
			return false
		}
		if len(want) > 0 {
			if _, mapped := want[0].(map[string]any); mapped {
				return b.eachContained(listed)
			}
		}
		if len(want) != len(listed) {
			return false
		}
		for i, element := range want {
			if !(baseline{want: element}).matches(listed[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		mapped, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for key, element := range want {
			held, found := mapped[key]
			if !found || !(baseline{want: element}).matches(held) {
				return false
			}
		}
		return true
	default:
		return b.scalarEqual(got)
	}
}

// eachContained reports whether every element of the baseline list has a
// matching element in the project's list, in any order.
func (b baseline) eachContained(listed []any) bool {
	want, ok := b.want.([]any)
	if !ok {
		return false
	}
	for _, element := range want {
		matched := slices.ContainsFunc(listed, (baseline{want: element}).matches)
		if !matched {
			return false
		}
	}
	return true
}

// scalarEqual compares the baseline scalar against a YAML scalar,
// normalizing int and float so 5 and 5.0 agree.
func (b baseline) scalarEqual(got any) bool {
	if wantNumber, ok := asNumber(b.want); ok {
		gotNumber, ok := asNumber(got)
		return ok && wantNumber == gotNumber
	}
	return b.want == got
}

// asNumber widens any YAML numeric to float64 for comparison.
func asNumber(number any) (float64, bool) {
	switch typed := number.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

// formatWant renders a baseline or observed fragment for a finding message.
func formatWant(fragment any) string {
	rendered, err := yaml.Marshal(fragment)
	if err != nil {
		return fmt.Sprintf("%v", fragment)
	}
	return strings.TrimSpace(string(rendered))
}

// readConfig loads and parses the project's golangci-lint configuration.
func readConfig(dir string) (map[string]any, error) {
	path, found := configPath(dir)
	if !found {
		return nil, fmt.Errorf("no golangci-lint config found in %s (looked for %s)",
			dir, strings.Join(configNames, ", "))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	document := map[string]any{}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	return document, nil
}

// configPath probes dir for a golangci-lint configuration file.
func configPath(dir string) (string, bool) {
	for _, name := range configNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

// modulePath reads the module path from dir's go.mod.
func modulePath(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading module path: %w", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if module, found := strings.CutPrefix(trimmed, "module "); found {
			return strings.TrimSpace(module), nil
		}
	}
	return "", errors.New("go.mod has no module line")
}
