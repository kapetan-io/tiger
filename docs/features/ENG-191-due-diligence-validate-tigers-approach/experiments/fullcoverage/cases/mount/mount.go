// Package mount reproduces github.com/containerd/containerd/v2@v2.1.4 core/mount/mount_unix.go:57 (UnmountRecursive):
// dedupe mountpoints through a set, then SliceStable by path length descending.
package mount

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"fullcoverage/cases/variant"
)

type Info struct{ Mountpoint string }

// Original is the shipped shape: equal-length targets unmount in map order.
func Original(mounts []Info) []string {
	targetSet := make(map[string]struct{})
	for _, m := range mounts {
		targetSet[m.Mountpoint] = struct{}{}
	}

	var targets []string
	for m := range targetSet {
		targets = append(targets, m)
	}

	// Make the deepest mount be first
	sort.SliceStable(targets, func(i, j int) bool {
		return len(targets[i]) > len(targets[j])
	})
	return targets
}

// S1 takes the sorted keys, then the original stable sort.
func S1(mounts []Info) []string {
	targetSet := make(map[string]struct{})
	for _, m := range mounts {
		targetSet[m.Mountpoint] = struct{}{}
	}

	targets := slices.Sorted(maps.Keys(targetSet))

	// Make the deepest mount be first
	sort.SliceStable(targets, func(i, j int) bool {
		return len(targets[i]) > len(targets[j])
	})
	return targets
}

// S3 keeps the collect loop and adds sort.Strings as the slice's first use.
func S3(mounts []Info) []string {
	targetSet := make(map[string]struct{})
	for _, m := range mounts {
		targetSet[m.Mountpoint] = struct{}{}
	}

	var targets []string
	for m := range targetSet {
		targets = append(targets, m)
	}
	sort.Strings(targets)

	// Make the deepest mount be first
	sort.SliceStable(targets, func(i, j int) bool {
		return len(targets[i]) > len(targets[j])
	})
	return targets
}

// S4 makes the comparator total; the map loop still fires.
func S4(mounts []Info) []string {
	targetSet := make(map[string]struct{})
	for _, m := range mounts {
		targetSet[m.Mountpoint] = struct{}{}
	}

	var targets []string
	for m := range targetSet {
		targets = append(targets, m)
	}

	// Make the deepest mount be first
	slices.SortFunc(targets, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), strings.Compare(a, b))
	})
	return targets
}

// S6 drops the set: sort and compact the mountpoints directly.
func S6(mounts []Info) []string {
	var targets []string
	for _, m := range mounts {
		targets = append(targets, m.Mountpoint)
	}
	slices.Sort(targets)
	targets = slices.Compact(targets)

	// Make the deepest mount be first
	sort.SliceStable(targets, func(i, j int) bool {
		return len(targets[i]) > len(targets[j])
	})
	return targets
}

func fixture() []Info {
	return []Info{{"/a/b"}, {"/a/c"}, {"/a/d"}, {"/a"}, {"/a/b/x"}, {"/a/c/y"}, {"/a/b"}, {"/a/e"}}
}

func render(f func([]Info) []string) func() string {
	return func() string { return fmt.Sprint(f(fixture())) }
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "S1", Run: render(S1)},
	{Name: "S3", Run: render(S3)},
	{Name: "S4", Run: render(S4)},
	{Name: "S6", Run: render(S6)},
}
