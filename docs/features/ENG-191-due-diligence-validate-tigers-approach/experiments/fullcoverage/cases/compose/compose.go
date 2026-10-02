// Package compose reproduces github.com/compose-spec/compose-go/v2@v2.9.0 types/types.go:167 (NetworksByPriority): a
// tie-free comparator, priority descending then name, where name is the map key.
package compose

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"

	"fullcoverage/cases/variant"
)

type ServiceNetworkConfig struct{ Priority int }

// Original is the shipped shape.
func Original(networks map[string]*ServiceNetworkConfig) []string {
	type key struct {
		name     string
		priority int
	}
	var keys []key
	for k, v := range networks {
		priority := 0
		if v != nil {
			priority = v.Priority
		}
		keys = append(keys, key{
			name:     k,
			priority: priority,
		})
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].priority == keys[j].priority {
			return keys[i].name < keys[j].name
		}
		return keys[i].priority > keys[j].priority
	})
	var sorted []string
	for _, k := range keys {
		sorted = append(sorted, k.name)
	}
	return sorted
}

// S1 ranges the sorted names; the comparator stays as written.
func S1(networks map[string]*ServiceNetworkConfig) []string {
	type key struct {
		name     string
		priority int
	}
	var keys []key
	for _, k := range slices.Sorted(maps.Keys(networks)) {
		priority := 0
		if v := networks[k]; v != nil {
			priority = v.Priority
		}
		keys = append(keys, key{
			name:     k,
			priority: priority,
		})
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].priority == keys[j].priority {
			return keys[i].name < keys[j].name
		}
		return keys[i].priority > keys[j].priority
	})
	var sorted []string
	for _, k := range keys {
		sorted = append(sorted, k.name)
	}
	return sorted
}

// S6 drops the key struct: sorted names, then a stable sort by priority read from the map.
func S6(networks map[string]*ServiceNetworkConfig) []string {
	priority := func(name string) int {
		if v := networks[name]; v != nil {
			return v.Priority
		}
		return 0
	}
	sorted := slices.Sorted(maps.Keys(networks))
	slices.SortStableFunc(sorted, func(a, b string) int { return cmp.Compare(priority(b), priority(a)) })
	return sorted
}

// S4 rewrites the comparator with cmp.Or; the map loop still fires.
func S4(networks map[string]*ServiceNetworkConfig) []string {
	type key struct {
		name     string
		priority int
	}
	var keys []key
	for k, v := range networks {
		priority := 0
		if v != nil {
			priority = v.Priority
		}
		keys = append(keys, key{name: k, priority: priority})
	}
	slices.SortFunc(keys, func(a, b key) int {
		return cmp.Or(cmp.Compare(b.priority, a.priority), cmp.Compare(a.name, b.name))
	})
	var sorted []string
	for _, k := range keys {
		sorted = append(sorted, k.name)
	}
	return sorted
}

func fixture() map[string]*ServiceNetworkConfig {
	return map[string]*ServiceNetworkConfig{
		"front": {Priority: 10}, "back": {Priority: 10}, "admin": nil, "db": {}, "cache": {Priority: 5}, "logs": {Priority: 5},
	}
}

func render(f func(map[string]*ServiceNetworkConfig) []string) func() string {
	return func() string { return fmt.Sprint(f(fixture())) }
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "S1", Run: render(S1)},
	{Name: "S4", Run: render(S4)},
	{Name: "S6", Run: render(S6)},
}
