package changed

import (
	"fmt"
	"sort"
	"strings"
)

// Change is an edge that exists in only one version: Added when head has it
// and base does not, removed otherwise.
type Change struct {
	Edge
	Added bool
}

// Diff returns the edges added and removed between base and head. An edge
// in both versions is unchanged even when the methods behind it differ.
func Diff(base, head Edges) []Change {
	var changes []Change
	for _, edge := range base.List() {
		if head[edge.Owner][edge.Target] == nil {
			changes = append(changes, Change{Edge: edge})
		}
	}
	for _, edge := range head.List() {
		if base[edge.Owner][edge.Target] == nil {
			changes = append(changes, Change{Edge: edge, Added: true})
		}
	}
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Owner != changes[j].Owner {
			return changes[i].Owner < changes[j].Owner
		}
		return changes[i].Target < changes[j].Target
	})
	return changes
}

// Report renders changes as text grouped by owner, one line per edge:
//
//	web.Handler
//	  + example.com/fakedb.Pool: ServeHTTP calls Exec
func Report(changes []Change) string {
	var out strings.Builder
	owner := ""
	for _, change := range changes {
		if change.Owner != owner {
			owner = change.Owner
			fmt.Fprintln(&out, owner)
		}
		sign := "-"
		if change.Added {
			sign = "+"
		}
		fmt.Fprintf(&out, "  %s %s: %s\n", sign, change.Target, strings.Join(change.Calls, "; "))
	}
	return out.String()
}

// Mermaid renders only the changed edges as a Mermaid flowchart: added
// edges solid, removed edges dotted.
func Mermaid(changes []Change) string {
	var out strings.Builder
	fmt.Fprintln(&out, "graph LR")
	ids := map[string]string{}
	node := func(name string) string {
		if id, ok := ids[name]; ok {
			return id
		}
		id := fmt.Sprintf("n%d", len(ids))
		ids[name] = id
		fmt.Fprintf(&out, "  %s[\"%s\"]\n", id, name)
		return id
	}
	for _, change := range changes {
		from, to := node(change.Owner), node(change.Target)
		if change.Added {
			fmt.Fprintf(&out, "  %s -->|+| %s\n", from, to)
		} else {
			fmt.Fprintf(&out, "  %s -.->|-| %s\n", from, to)
		}
	}
	return out.String()
}
