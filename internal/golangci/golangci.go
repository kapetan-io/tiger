// Package golangci audits the other half of the dialect: the auto rules
// delegated to golangci-lint. Verify checks a project's configuration
// against the registry's auto-rule baseline; Generate and Init derive that
// baseline config from the registry, so a rule cannot be registered without
// being both generated and audited.
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

// matchOp classifies a node of the comparison tree matches builds: a leaf
// verdict, a conjunction, or a disjunction.
type matchOp int

const (
	matchLeaf matchOp = iota
	matchAll
	matchAny
)

// matchNode is one node of the and/or comparison tree.
type matchNode struct {
	op       matchOp
	verdict  bool
	children []*matchNode
}

// matchTask pairs a baseline fragment and a project fragment with the tree
// node their comparison fills.
type matchTask struct {
	want any
	got  any
	node *matchNode
}

// matchState carries the tree build: the task worklist and every created
// node in creation order, so evaluation can run bottom-up without
// recursion (children always sit after their parents).
type matchState struct {
	work  []matchTask
	nodes []*matchNode
}

// matches compares a project's YAML fragment against the baseline. Scalars
// and scalar lists must match exactly; a list of maps must contain every
// baseline element (extra elements pass); a map must hold every baseline
// key (extra keys pass). The comparison builds an and/or tree with an
// index-advancing worklist, then evaluates it bottom-up — no recursion.
func (b baseline) matches(got any) bool {
	root := &matchNode{}
	tree := &matchState{nodes: []*matchNode{root}}
	tree.work = []matchTask{{want: b.want, got: got, node: root}}
	for i := 0; i < len(tree.work); i++ {
		tree.expand(i)
	}
	for i := len(tree.nodes) - 1; i >= 0; i-- {
		evalNode(tree.nodes[i])
	}
	return root.verdict
}

// expand fills one task's node: a leaf verdict, or a connective whose
// child tasks land back on the worklist.
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

// expandContainment handles a baseline list of maps: every baseline
// element (AND) must match some project element (OR), in any order.
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

// containment reports whether a baseline list compares by containment: a
// non-empty list whose first element is a map.
func containment(want []any) bool {
	if len(want) == 0 {
		return false
	}
	_, mapped := want[0].(map[string]any)
	return mapped
}

// child creates one node under parent, registered for evaluation.
func (s *matchState) child(parent *matchNode) *matchNode {
	created := &matchNode{}
	parent.children = append(parent.children, created)
	s.nodes = append(s.nodes, created)
	return created
}

// push queues one comparison task.
func (s *matchState) push(task matchTask) {
	s.work = append(s.work, task)
}

// evalNode computes one node's verdict from its already-evaluated
// children: a conjunction of none is true, a disjunction of none false.
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

// configPath probes dir for a golangci-lint configuration file. One
// directory listing answers every candidate name; an unreadable dir means
// no config, same as a failed per-file probe.
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
