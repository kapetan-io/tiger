// Package checker reproduces golang.org/x/tools@v0.6.0 go/analysis/internal/checker/checker.go:586:
// the key is *action (a pointer); actions print by duration descending, ties in map order.
package checker

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"fullcoverage/cases/variant"
)

type action struct {
	analyzer, pkg string
	duration      time.Duration
}

func (a *action) String() string { return a.analyzer + "@" + a.pkg }

// Original is the shipped shape.
func Original(printed map[*action]bool) string {
	var buf strings.Builder
	var all []*action
	var total time.Duration
	for act := range printed {
		all = append(all, act)
		total += act.duration
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].duration > all[j].duration
	})
	for _, act := range all {
		fmt.Fprintf(&buf, "%s\t%s\n", act.duration, act)
	}
	fmt.Fprintf(&buf, "total %s\n", total)
	return buf.String()
}

// S2 sorts the pointer keys by a key-only comparator over String, then stable by duration.
// It escapes TS-T02; it is deterministic only because String is unique per action.
func S2(printed map[*action]bool) string {
	var buf strings.Builder
	var total time.Duration
	byName := func(a, b *action) int { return strings.Compare(a.String(), b.String()) }
	all := slices.SortedFunc(maps.Keys(printed), byName)
	for _, act := range all {
		total += act.duration
	}
	slices.SortStableFunc(all, func(a, b *action) int { return cmp.Compare(b.duration, a.duration) })
	for _, act := range all {
		fmt.Fprintf(&buf, "%s\t%s\n", act.duration, act)
	}
	fmt.Fprintf(&buf, "total %s\n", total)
	return buf.String()
}

// S4 keeps the map loop and makes the comparator total over (duration, String).
func S4(printed map[*action]bool) string {
	var buf strings.Builder
	var all []*action
	var total time.Duration
	for act := range printed {
		all = append(all, act)
		total += act.duration
	}
	slices.SortFunc(all, func(a, b *action) int {
		return cmp.Or(cmp.Compare(b.duration, a.duration), strings.Compare(a.String(), b.String()))
	})
	for _, act := range all {
		fmt.Fprintf(&buf, "%s\t%s\n", act.duration, act)
	}
	fmt.Fprintf(&buf, "total %s\n", total)
	return buf.String()
}

// S5 keeps the order beside the set: the caller records each action as it marks it printed,
// so the slice holds the (deterministic) visit order and no map is ranged.
func S5(printedOrder []*action) string {
	var buf strings.Builder
	all := slices.Clone(printedOrder)
	var total time.Duration
	for _, act := range all {
		total += act.duration
	}
	slices.SortStableFunc(all, func(a, b *action) int { return cmp.Compare(b.duration, a.duration) })
	for _, act := range all {
		fmt.Fprintf(&buf, "%s\t%s\n", act.duration, act)
	}
	fmt.Fprintf(&buf, "total %s\n", total)
	return buf.String()
}

func fixture() ([]*action, map[*action]bool) {
	var order []*action
	printed := map[*action]bool{}
	for i := range 9 {
		act := &action{analyzer: fmt.Sprintf("a%d", i%3), pkg: fmt.Sprintf("p%d", i/3), duration: time.Duration(i%2) * time.Millisecond}
		order = append(order, act)
		printed[act] = true
	}
	return order, printed
}

func render(f func(map[*action]bool) string) func() string {
	return func() string {
		_, printed := fixture()
		return f(printed)
	}
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "S2", Run: render(S2)},
	{Name: "S4", Run: render(S4)},
	{Name: "S5", Run: func() string {
		order, _ := fixture()
		return S5(order)
	}},
}
