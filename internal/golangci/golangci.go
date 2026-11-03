// Package golangci is tiger golangci. It audits the other half of the rule
// set: the auto rules delegated to golangci-lint. Verify checks a project's
// configuration against the auto-rule baseline; Generate and Init derive that
// baseline config from internal/rules/auto, so a rule cannot be registered
// without being both generated and audited. Run is the command over them.
package golangci

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/rules/auto"
	"github.com/kapetan-io/tiger/internal/worklist"
)

var ErrConfigExists = errors.New(
	"a golangci-lint config already exists; run tiger golangci to audit it",
)

var configNames = []string{".golangci.yml", ".golangci.yaml"}

type Finding struct {
	RuleID  string
	Linter  string
	Message string
}

type baseline struct {
	want any
}

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
	for _, rule := range auto.AutoRules() {
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

func verifyRule(document map[string]any, rule auto.AutoRule, module string) []Finding {
	if !enabled(document, rule) {
		return []Finding{{
			RuleID: rule.RuleID,
			Linter: rule.Linter,
			Message: fmt.Sprintf(
				"%s: %s is missing from %s.enable, so nothing enforces the rule %q — "+
					"add %s to %s.enable",
				rule.RuleID,
				rule.Linter,
				rule.Section,
				rule.Title,
				rule.Linter,
				rule.Section,
			),
		}}
	}
	findings := []Finding{}
	for _, setting := range rule.Settings {
		joined := strings.Join(setting.Path, ".")
		want := auto.SubstituteModule(setting.Want, module)
		got, found := lookup(document, setting.Path)
		if !found {
			findings = append(findings, Finding{
				RuleID: rule.RuleID,
				Linter: rule.Linter,
				Message: fmt.Sprintf(
					"%s: %s isn't set, and tiger's baseline for the rule %q needs it — "+
						"set %s to %v%s",
					rule.RuleID, joined, rule.Title, joined, formatWant(want),
					reasonSuffix(setting)),
			})
			continue
		}
		if setting.Contains {
			if absent := strings.Join((baseline{want: want}).missingFrom(got), ", "); absent != "" {
				findings = append(findings, Finding{
					RuleID: rule.RuleID,
					Linter: rule.Linter,
					Message: fmt.Sprintf(
						"%[1]s: %[2]s doesn't include %[3]s, which tiger's baseline for the "+
							"rule %[4]q needs — add %[3]s to %[2]s%[5]s",
						rule.RuleID, joined, absent, rule.Title, reasonSuffix(setting)),
				})
			}
			continue
		}
		if !(baseline{want: want}).matches(got) {
			findings = append(findings, Finding{
				RuleID: rule.RuleID,
				Linter: rule.Linter,
				Message: fmt.Sprintf(
					"%s: %s is %v, but tiger's baseline for the rule %q is %v — set it to %v "+
						"(tiger compares exactly, so a stricter value also fails)",
					rule.RuleID,
					joined,
					formatWant(got),
					rule.Title,
					formatWant(want),
					formatWant(want),
				),
			})
		}
	}
	return findings
}

func reasonSuffix(setting auto.Setting) string {
	if setting.Reason == "" {
		return ""
	}
	return " (" + setting.Reason + ")"
}

func (b baseline) missingFrom(got any) []string {
	held := map[string]bool{}
	if listed, ok := got.([]any); ok {
		for _, element := range listed {
			held[fmt.Sprint(element)] = true
		}
	}
	required, ok := b.want.([]any)
	if !ok {
		return nil
	}
	absent := []string{}
	for _, element := range required {
		if !held[fmt.Sprint(element)] {
			absent = append(absent, fmt.Sprint(element))
		}
	}
	return absent
}

func enabled(document map[string]any, rule auto.AutoRule) bool {
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
	if rule.Section != auto.SectionLinters || !auto.StandardLinter(rule.Linter) {
		return false
	}
	group, found := lookup(document, []string{auto.SectionLinters, "default"})
	return found && group == "standard"
}

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

type matchOp int

const (
	matchLeaf matchOp = iota
	matchAll
	matchAny
)

type matchNode struct {
	op       matchOp
	verdict  bool
	children []*matchNode
}

type matchTask struct {
	want any
	got  any
	node *matchNode
}

type matchState struct {
	work  []matchTask
	nodes []*matchNode
}

func (b baseline) matches(got any) bool {
	root := &matchNode{}
	tree := &matchState{nodes: []*matchNode{root}}
	tree.work = []matchTask{{want: b.want, got: got, node: root}}
	i := 0
	for ; i < len(tree.work) && i < worklist.NodesMax; i++ {
		tree.expand(i)
	}
	assert.Ok(i == len(tree.work), "matches: worklist reached worklist.NodesMax")
	for i := len(tree.nodes) - 1; i >= 0; i-- {
		evalNode(tree.nodes[i])
	}
	return root.verdict
}

func (s *matchState) expand(i int) {
	task := s.work[i]
	switch want := task.want.(type) {
	case []any:
		listed, ok := task.got.([]any)
		if !ok {
			task.node.op = matchLeaf
			return
		}
		if containment(want) {
			s.expandContainment(task, listed)
			return
		}
		if len(want) != len(listed) {
			task.node.op = matchLeaf
			return
		}
		task.node.op = matchAll
		for at, element := range want {
			s.push(matchTask{want: element, got: listed[at], node: s.child(task.node)})
		}
	case map[string]any:
		mapped, ok := task.got.(map[string]any)
		if !ok {
			task.node.op = matchLeaf
			return
		}
		task.node.op = matchAll
		for _, key := range slices.Sorted(maps.Keys(want)) {
			held, found := mapped[key]
			if !found {
				s.child(task.node).op = matchLeaf
				continue
			}
			s.push(matchTask{want: want[key], got: held, node: s.child(task.node)})
		}
	default:
		task.node.op = matchLeaf
		task.node.verdict = (baseline{want: task.want}).scalarEqual(task.got)
	}
}

func (s *matchState) expandContainment(task matchTask, listed []any) {
	want, ok := task.want.([]any)
	if !ok {
		task.node.op = matchLeaf
		return
	}
	task.node.op = matchAll
	for _, element := range want {
		anyOf := s.child(task.node)
		anyOf.op = matchAny
		for _, candidate := range listed {
			s.push(matchTask{want: element, got: candidate, node: s.child(anyOf)})
		}
	}
}

func containment(want []any) bool {
	if len(want) == 0 {
		return false
	}
	_, mapped := want[0].(map[string]any)
	return mapped
}

func (s *matchState) child(parent *matchNode) *matchNode {
	created := &matchNode{}
	parent.children = append(parent.children, created)
	s.nodes = append(s.nodes, created)
	return created
}

func (s *matchState) push(task matchTask) {
	s.work = append(s.work, task)
}

func evalNode(node *matchNode) {
	switch node.op {
	case matchLeaf:
		return
	case matchAll:
		node.verdict = true
		for _, held := range node.children {
			if !held.verdict {
				node.verdict = false
			}
		}
	case matchAny:
		node.verdict = false
		for _, held := range node.children {
			if held.verdict {
				node.verdict = true
			}
		}
	default:
		assert.Unreachable("matchOp: unhandled value")
	}
}

func (b baseline) scalarEqual(got any) bool {
	if wantNumber, ok := asNumber(b.want); ok {
		gotNumber, ok := asNumber(got)
		return ok && wantNumber == gotNumber
	}
	return b.want == got
}

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

func formatWant(fragment any) string {
	rendered, err := yaml.Marshal(fragment)
	if err != nil {
		return fmt.Sprintf("%v", fragment)
	}
	return strings.TrimSpace(string(rendered))
}

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

func configPath(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	present := map[string]bool{}
	for _, entry := range entries {
		present[entry.Name()] = true
	}
	for _, name := range configNames {
		if present[name] {
			return filepath.Join(dir, name), true
		}
	}
	return "", false
}

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
