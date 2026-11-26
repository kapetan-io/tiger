package golangci

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/rules/auto"
	"github.com/kapetan-io/tiger/internal/worklist"
)

var changeableKeys = []string{
	"linters.enable",
	"formatters.enable",
	"linters.default",
	"issues.uniq-by-line",
	"linters.exclusions.rules",
	"linters.settings",
	"formatters.settings",
}

var harmlessValues = map[string][]any{
	"linters.exclusions.generated": {"strict", "disable"},
	"linters.exclusions.presets":   {"common-false-positives"},
}

type keyTask struct {
	path    []string
	want    any
	got     any
	wanted  bool
	present bool
}

type keyAudit struct {
	work     []keyTask
	owned    map[string]bool
	findings []Finding
}

func verifyKeys(document map[string]any, module string) []Finding {
	audit := &keyAudit{owned: ruleOwnedKeys()}
	audit.work = []keyTask{{
		want: baselineDocument(module), got: document, wanted: true, present: true,
	}}
	i := 0
	for ; i < len(audit.work) && i < worklist.NodesMax; i++ {
		audit.expand(audit.work[i])
	}
	assert.Ok(i == len(audit.work), "verifyKeys: worklist reached worklist.NodesMax")
	return audit.findings
}

func ruleOwnedKeys() map[string]bool {
	owned := map[string]bool{}
	autoRules := auto.AutoRules()
	for i := range autoRules {
		for j := range autoRules[i].Settings {
			owned[strings.Join(autoRules[i].Settings[j].Path, ".")] = true
		}
	}
	return owned
}

func (a *keyAudit) expand(task keyTask) {
	joined := strings.Join(task.path, ".")
	if slices.Contains(changeableKeys, joined) {
		return
	}
	task.present = task.present && task.got != nil
	if a.owned[joined] {
		a.addedElements(task, joined)
		return
	}
	wantMap, wantIsMap := task.want.(map[string]any)
	gotMap, gotIsMap := task.got.(map[string]any)
	wantBranches := wantIsMap || !task.wanted
	gotBranches := gotIsMap || !task.present
	if wantBranches && gotBranches {
		for _, key := range slices.Sorted(maps.Keys(union(wantMap, gotMap))) {
			want, wanted := wantMap[key]
			got, held := gotMap[key]
			a.work = append(a.work, keyTask{
				path: append(slices.Clone(task.path), key),
				want: want, got: got, wanted: wanted, present: held,
			})
		}
		return
	}
	a.compareLeaf(task, joined)
}

func (a *keyAudit) compareLeaf(task keyTask, joined string) {
	_, wantIsMap := task.want.(map[string]any)
	switch {
	case !task.wanted && slices.Contains(harmlessValues[joined], task.got):
		return
	case !task.wanted:
		a.report(fmt.Sprintf("%[1]s is set, but tiger's baseline doesn't set it — remove %[1]s",
			joined))
	case !task.present:
		a.report(fmt.Sprintf("%[1]s isn't set, but tiger's baseline sets it — set %[1]s to %[2]v",
			joined, formatWant(task.want)))
	case wantIsMap:
		a.report(fmt.Sprintf("%[1]s isn't laid out as in tiger's baseline, which holds keys "+
			"under it — copy %[1]s from tiger golangci --print", joined))
	case !(baseline{want: task.want}).sameLeaf(task.got):
		a.report(fmt.Sprintf("%s is %v, but tiger's baseline is %v — set it to %v",
			joined, describe(task.got), formatWant(task.want), formatWant(task.want)))
	}
}

func (b baseline) sameLeaf(got any) bool {
	if b.matches(got) {
		return true
	}
	text, isText := b.want.(string)
	return isText && text == fmt.Sprint(got)
}

func describe(got any) string {
	switch got.(type) {
	case map[string]any, map[any]any:
		return "a block of keys"
	default:
		return formatWant(got)
	}
}

func (a *keyAudit) addedElements(task keyTask, joined string) {
	listed, ok := task.got.([]any)
	if !ok {
		return
	}
	wanted, _ := task.want.([]any)
	for _, element := range listed {
		if slices.Contains(wanted, element) || slices.Contains(harmlessValues[joined], element) {
			continue
		}
		a.report(fmt.Sprintf("%[1]s lists %[2]v, which tiger's baseline doesn't — "+
			"remove %[2]v from %[1]s", joined, element))
	}
}

func (a *keyAudit) report(message string) {
	a.findings = append(a.findings, Finding{Message: message})
}

func union(documents ...map[string]any) map[string]any {
	keys := map[string]any{}
	for _, document := range documents {
		maps.Copy(keys, document)
	}
	return keys
}
