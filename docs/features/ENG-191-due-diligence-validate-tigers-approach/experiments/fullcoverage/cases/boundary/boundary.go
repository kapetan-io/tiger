// Package boundary probes each soundness boundary of full coverage on data
// with ties: every rejected shape here can find two distinguishable
// elements equal, and the fixture makes it happen, so the rejection is
// needed rather than merely cautious. Accepted shapes sit beside them on the
// same data. The last functions are the expected false positives and the
// known misses the analyzer leaves open.
package boundary

import (
	"bytes"
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"fullcoverage/cases/variant"
)

type Hash [4]byte

func hashes() map[Hash]bool {
	m := map[Hash]bool{}
	for i := range 12 {
		m[Hash{byte(i % 3), byte(i)}] = true
	}
	return m
}

// PartialArray compares one byte of the key.
func PartialArray(m map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(m), func(a, b Hash) int { return cmp.Compare(a[0], b[0]) })
}

// WholeArray compares the whole key.
func WholeArray(m map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(m), func(a, b Hash) int { return bytes.Compare(a[:], b[:]) })
}

type Member struct{ Team, ID string }

func members() map[Member]bool {
	m := map[Member]bool{}
	for i := range 12 {
		m[Member{Team: fmt.Sprintf("t%d", i%3), ID: fmt.Sprintf("u%02d", i)}] = true
	}
	return m
}

// OneField compares one field of the key.
func OneField(m map[Member]bool) []Member {
	return slices.SortedFunc(maps.Keys(m), func(a, b Member) int { return cmp.Compare(a.Team, b.Team) })
}

// EveryField compares both fields.
func EveryField(m map[Member]bool) []Member {
	return slices.SortedFunc(maps.Keys(m), func(a, b Member) int {
		return cmp.Or(cmp.Compare(a.Team, b.Team), cmp.Compare(a.ID, b.ID))
	})
}

// Label prints only the first byte, so it is not injective.
func (h Hash) Label() string { return fmt.Sprintf("%02x", h[0]) }

// StringMethod compares through a method call.
func StringMethod(m map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(m), func(a, b Hash) int { return strings.Compare(a.Label(), b.Label()) })
}

type record struct {
	name string
	id   int
}

func labelled() map[*record]string {
	m := map[*record]string{}
	for i := range 8 {
		m[&record{name: "same", id: 1}] = fmt.Sprintf("label%d", i)
	}
	return m
}

func labels(m map[*record]string, keys []*record) string {
	var out []string
	for _, k := range keys {
		out = append(out, m[k])
	}
	return strings.Join(out, " ")
}

// PointerLeaf compares every field of the pointee; distinct pointers with
// equal pointees tie, and the labels they map to print in map order.
func PointerLeaf(m map[*record]string) string {
	keys := slices.SortedFunc(maps.Keys(m), func(a, b *record) int {
		return cmp.Or(cmp.Compare(a.name, b.name), cmp.Compare(a.id, b.id))
	})
	return labels(m, keys)
}

type Shape interface{ Area() int }

type Square struct{ Side int }

type Rect struct{ W, H int }

func (s Square) Area() int { return s.Side * s.Side }
func (r Rect) Area() int   { return r.W * r.H }

func shapes() map[Shape]bool {
	return map[Shape]bool{Square{2}: true, Rect{1, 4}: true, Rect{4, 1}: true, Rect{2, 2}: true, Square{3}: true, Rect{1, 9}: true}
}

// InterfaceLeaf compares through the interface's method.
func InterfaceLeaf(m map[Shape]bool) []Shape {
	return slices.SortedFunc(maps.Keys(m), func(a, b Shape) int { return cmp.Compare(a.Area(), b.Area()) })
}

type Score struct {
	Name  string
	Value float64
}

func scores() map[string]Score {
	negative := math.Copysign(0, -1)
	m := map[string]Score{}
	for i := range 8 {
		value := 0.0
		if i%2 == 0 {
			value = negative
		}
		m[fmt.Sprintf("k%d", i)] = Score{Name: "a", Value: value}
	}
	return m
}

// FloatLeaf compares every field, a float among them: -0 and +0 tie.
func FloatLeaf(m map[string]Score) []Score {
	return slices.SortedFunc(maps.Values(m), func(a, b Score) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Value, b.Value))
	})
}

type Flag struct {
	Name string
	On   bool
}

func flags() map[string]Flag {
	m := map[string]Flag{}
	for i := range 8 {
		m[fmt.Sprintf("k%d", i)] = Flag{Name: "a", On: i%2 == 0}
	}
	return m
}

// BoolLeaf compares every field but the bool, which cmp.Compare can't take.
func BoolLeaf(m map[string]Flag) []Flag {
	return slices.SortedFunc(maps.Values(m), func(a, b Flag) int { return cmp.Compare(a.Name, b.Name) })
}

type Stamp struct {
	At   time.Time
	Name string
}

func stamps() map[Stamp]bool {
	instant := time.Unix(1_700_000_000, 0)
	m := map[Stamp]bool{}
	for i := range 6 {
		m[Stamp{At: instant.In(time.FixedZone(fmt.Sprintf("Z%d", i), 0)), Name: "a"}] = true
	}
	return m
}

// TimeField compares time.Time with its Compare: one instant in different
// locations compares equal and prints differently.
func TimeField(m map[Stamp]bool) []Stamp {
	return slices.SortedFunc(maps.Keys(m), func(a, b Stamp) int {
		return cmp.Or(a.At.Compare(b.At), strings.Compare(a.Name, b.Name))
	})
}

type item struct {
	name  string
	count int
}

func counts() map[string]int {
	return map[string]int{"a": 3, "b": 3, "c": 3, "d": 2, "e": 2, "f": 1, "g": 1, "h": 1}
}

// AsymmetricTieBreak's first comparison is not antisymmetric, so the sort
// sees an inconsistent order.
func AsymmetricTieBreak(m map[string]int) []item {
	var items []item
	for name, count := range m {
		items = append(items, item{name, count})
	}
	slices.SortFunc(items, func(x, y item) int {
		return cmp.Or(cmp.Compare(x.count, y.count+1), strings.Compare(x.name, y.name), cmp.Compare(x.count, y.count))
	})
	return items
}

// CallingTieBreak's first comparison calls a function with state.
func CallingTieBreak(m map[string]int) []string {
	calls := 0
	weight := func(string) int {
		calls++
		return calls % 2
	}
	var names []string
	for name := range m {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(weight(a), weight(b)), strings.Compare(a, b))
	})
	return names
}

// TotalItems is the accepted shape: count descending, then name.
func TotalItems(m map[string]int) []item {
	var items []item
	for name, count := range m {
		items = append(items, item{name, count})
	}
	slices.SortFunc(items, func(x, y item) int {
		return cmp.Or(-cmp.Compare(x.count, y.count), strings.Compare(x.name, y.name))
	})
	return items
}

type ranked struct {
	name string
	rank int
}

// VisitRank puts a counter in the element; every field is compared, but
// the ranks follow visit order.
func VisitRank(m map[string]int) []ranked {
	var out []ranked
	n := 0
	for name := range m {
		n++
		out = append(out, ranked{name, n})
	}
	slices.SortFunc(out, func(a, b ranked) int {
		return cmp.Or(cmp.Compare(a.name, b.name), cmp.Compare(a.rank, b.rank))
	})
	return out
}

// GroupByValue builds per-value slices in map order; tiger passes it.
func GroupByValue(m map[string]int) string {
	groups := map[int][]string{}
	for name, count := range m {
		groups[count] = append(groups[count], name)
	}
	var out []string
	for _, count := range slices.Sorted(maps.Keys(groups)) {
		out = append(out, fmt.Sprint(count, groups[count]))
	}
	return strings.Join(out, " ")
}

// CollectEscape collects keys with slices.Collect and never sorts them.
func CollectEscape(m map[string]int) []string {
	return slices.Collect(maps.Keys(m))
}

type Base struct{ Zone string }

type Host struct {
	Base
	Name string
}

func hosts() map[Host]bool {
	m := map[Host]bool{}
	for i := range 12 {
		m[Host{Base{fmt.Sprintf("z%d", i%3)}, fmt.Sprintf("h%02d", i)}] = true
	}
	return m
}

// PromotedFields covers Zone through promotion: accepted.
func PromotedFields(m map[Host]bool) []Host {
	return slices.SortedFunc(maps.Keys(m), func(a, b Host) int {
		return cmp.Or(cmp.Compare(a.Zone, b.Zone), cmp.Compare(a.Name, b.Name))
	})
}

// StoredComparator is total but held in a variable: an expected false
// positive.
func StoredComparator(m map[Hash]bool) []Hash {
	byBytes := func(a, b Hash) int { return bytes.Compare(a[:], b[:]) }
	return slices.SortedFunc(maps.Keys(m), byBytes)
}

// IndexComparator is total but uses sort.Slice: an expected false positive.
func IndexComparator(m map[string]int) []string {
	var names []string
	for name := range m {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}

// Invert is a known miss: two keys with one value write one slot, and the
// last visited wins.
func Invert(m map[string]int) map[int]string {
	inverse := map[int]string{}
	for name, count := range m {
		inverse[count] = name
	}
	return inverse
}

var Variants = []variant.Variant{
	{Name: "PartialArray", Run: func() string { return fmt.Sprint(PartialArray(hashes())) }},
	{Name: "WholeArray", Run: func() string { return fmt.Sprint(WholeArray(hashes())) }},
	{Name: "OneField", Run: func() string { return fmt.Sprint(OneField(members())) }},
	{Name: "EveryField", Run: func() string { return fmt.Sprint(EveryField(members())) }},
	{Name: "StringMethod", Run: func() string { return fmt.Sprint(StringMethod(hashes())) }},
	{Name: "PointerLeaf", Run: func() string { return PointerLeaf(labelled()) }},
	{Name: "InterfaceLeaf", Run: func() string { return fmt.Sprintf("%#v", InterfaceLeaf(shapes())) }},
	{Name: "FloatLeaf", Run: func() string { return fmt.Sprint(FloatLeaf(scores())) }},
	{Name: "BoolLeaf", Run: func() string { return fmt.Sprint(BoolLeaf(flags())) }},
	{Name: "TimeField", Run: func() string { return fmt.Sprint(TimeField(stamps())) }},
	{Name: "AsymmetricTieBreak", Run: func() string { return fmt.Sprint(AsymmetricTieBreak(counts())) }},
	{Name: "CallingTieBreak", Run: func() string { return fmt.Sprint(CallingTieBreak(counts())) }},
	{Name: "TotalItems", Run: func() string { return fmt.Sprint(TotalItems(counts())) }},
	{Name: "VisitRank", Run: func() string { return fmt.Sprint(VisitRank(counts())) }},
	{Name: "GroupByValue", Run: func() string { return GroupByValue(counts()) }},
	{Name: "CollectEscape", Run: func() string { return fmt.Sprint(CollectEscape(counts())) }},
	{Name: "PromotedFields", Run: func() string { return fmt.Sprint(PromotedFields(hosts())) }},
	{Name: "StoredComparator", Run: func() string { return fmt.Sprint(StoredComparator(hashes())) }},
	{Name: "IndexComparator", Run: func() string { return fmt.Sprint(IndexComparator(counts())) }},
	{Name: "Invert", Run: func() string { return fmt.Sprint(Invert(counts())) }},
}
