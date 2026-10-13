// Item 2, full coverage: slices.SortedFunc over maps.Keys or maps.Values,
// and slices.SortFunc or SortStableFunc after a collect loop, pass only when
// the comparator's comparisons together reach every leaf of the element.
// Each rejected case below names the boundary it tests.
package fixture

import (
	"bytes"
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

type Hash [20]byte

type OID struct {
	format uint8
	raw    [4]byte
}

// Compare covers every field of OID, so it earns the fact.
func (o OID) Compare(p OID) int { // want Compare:"total"
	return cmp.Or(cmp.Compare(o.format, p.format), bytes.Compare(o.raw[:], p.raw[:]))
}

func (o OID) String() string { return fmt.Sprintf("%x", o.raw[:1]) }

// Loose compares one byte of raw, so its Compare earns no fact.
type Loose struct {
	format uint8
	raw    [4]byte
}

func (o Loose) Compare(p Loose) int {
	return cmp.Or(cmp.Compare(o.format, p.format), cmp.Compare(o.raw[0], p.raw[0]))
}

// PtrKey's Compare has a pointer receiver, so it earns no fact.
type PtrKey struct{ name string }

func (p *PtrKey) Compare(o *PtrKey) int { return strings.Compare(p.name, o.name) }

type Pair struct {
	X int
	Y string
	_ bool // blank fields hold nothing; Go discards what is stored in them
}

type Flagged struct {
	X int
	B bool
}

type Node interface{ Pos() int }

type item struct {
	name  string
	count int
}

type Inner struct {
	A string
	B int
}

type Outer struct {
	In Inner
	ID int
}

type Base struct{ Zone string }

type Host struct {
	Base
	Name string
}

type PtrHost struct {
	*Base
	Name string
}

type Wrapped struct{ OID }

type Commit struct {
	ID  OID
	Seq int
}

type Linked struct {
	Name string
	Next *item
}

type Boxed struct {
	Name  string
	Value any
}

type Scored struct {
	Name  string
	Score float64
}

type Stamped struct {
	At   time.Time
	Name string
}

// --- accepted ---

func hashKeysBytes(m map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(m), func(a, b Hash) int { return bytes.Compare(a[:], b[:]) })
}

func hashKeysSlicesCompare(m map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(m), func(a, b Hash) int { return slices.Compare(a[:], b[:]) })
}

func shortArrayEveryIndex(m map[[2]byte]bool) [][2]byte {
	return slices.SortedFunc(maps.Keys(m), func(a, b [2]byte) int {
		return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1]))
	})
}

func oidKeysMethodExpr(m map[OID]bool) []OID {
	return slices.SortedFunc(maps.Keys(m), OID.Compare)
}

func oidKeysMethodCallDescending(m map[OID]bool) []OID {
	return slices.SortedFunc(maps.Keys(m), func(a, b OID) int { return b.Compare(a) })
}

func pairKeysEveryField(m map[Pair]int) []Pair {
	return slices.SortedFunc(maps.Keys(m), func(a, b Pair) int {
		return cmp.Or(cmp.Compare(b.X, a.X), -strings.Compare(a.Y, b.Y))
	})
}

func pairValuesEveryField(m map[string]Pair) []Pair {
	return slices.SortedStableFunc(maps.Values(m), func(a, b Pair) int {
		return cmp.Or(cmp.Compare(a.X, b.X), cmp.Compare(a.Y, b.Y))
	})
}

func nestedEveryLeaf(m map[Outer]bool) []Outer {
	return slices.SortedFunc(maps.Keys(m), func(a, b Outer) int {
		return cmp.Or(cmp.Compare(a.In.A, b.In.A), cmp.Compare(a.In.B, b.In.B), cmp.Compare(a.ID, b.ID))
	})
}

// a.Zone is promoted from Base; it covers the same leaf as a.Base.Zone.
func promotedFields(m map[Host]bool) []Host {
	return slices.SortedFunc(maps.Keys(m), func(a, b Host) int {
		return cmp.Or(cmp.Compare(a.Zone, b.Zone), cmp.Compare(a.Name, b.Name))
	})
}

func wrappedThroughField(m map[Wrapped]bool) []Wrapped {
	return slices.SortedFunc(maps.Keys(m), func(a, b Wrapped) int { return a.OID.Compare(b.OID) })
}

func methodOnField(m map[Commit]bool) []Commit {
	return slices.SortedFunc(maps.Keys(m), func(a, b Commit) int {
		return cmp.Or(cmp.Compare(a.Seq, b.Seq), a.ID.Compare(b.ID))
	})
}

func itemsTotalAfterCount(m map[string]int) []item {
	var items []item
	for name, count := range m {
		items = append(items, item{name, count})
	}
	slices.SortFunc(items, func(x, y item) int {
		return cmp.Or(-cmp.Compare(x.count, y.count), strings.Compare(x.name, y.name))
	})
	return items
}

// An earlier tie-break reading a map is fine: it mirrors and calls nothing.
func oidByTimeThenKey(m map[OID]bool, times map[OID]int64) []OID {
	var ordered []OID
	for oid := range m {
		ordered = append(ordered, oid)
	}
	slices.SortStableFunc(ordered, func(a, b OID) int {
		return cmp.Or(cmp.Compare(times[b], times[a]), b.Compare(a))
	})
	return ordered
}

func mountsDeepestFirst(set map[string]struct{}) []string {
	var targets []string
	for t := range set {
		targets = append(targets, t)
	}
	slices.SortFunc(targets, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), strings.Compare(a, b))
	})
	return targets
}

// A float earlier tie-break is a valid weak order; the later key covers.
func scoreThenName(m map[string]float64) []string {
	var names []string
	for name := range m {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(m[b], m[a]), strings.Compare(a, b))
	})
	return names
}

func copyAll(m map[string]int) map[string]int { return maps.Collect(maps.All(m)) }

func sortedOrdered(m map[string]int) []string { return slices.Sorted(maps.Keys(m)) }

// --- rejected ---

// Partial projection: one byte of the array.
func hashKeysPartial(m map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(m), func(a, b Hash) int { return cmp.Compare(a[0], b[0]) }) // want `TS-T02: this iterator`
}

// Partial projection: a sub-slice is not the whole array.
func hashKeysPrefix(m map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(m), func(a, b Hash) int { return bytes.Compare(a[:4], b[:4]) }) // want `TS-T02: this iterator`
}

// Partial projection: one field of the struct.
func pairKeysOneField(m map[Pair]int) []Pair {
	return slices.SortedFunc(maps.Keys(m), func(a, b Pair) int { return cmp.Compare(a.X, b.X) }) // want `TS-T02: this iterator`
}

// Partial projection: one leaf of a nested struct.
func nestedMissingLeaf(m map[Outer]bool) []Outer {
	return slices.SortedFunc(maps.Keys(m), func(a, b Outer) int { // want `TS-T02: this iterator`
		return cmp.Or(cmp.Compare(a.In.A, b.In.A), cmp.Compare(a.ID, b.ID))
	})
}

// A method call: String covers only raw[:1].
func oidKeysString(m map[OID]bool) []OID {
	return slices.SortedFunc(maps.Keys(m), func(a, b OID) int { return strings.Compare(a.String(), b.String()) }) // want `TS-T02: this iterator`
}

// A Compare method that does not cover every field earns no fact.
func looseKeysMethod(m map[Loose]bool) []Loose {
	return slices.SortedFunc(maps.Keys(m), Loose.Compare) // want `TS-T02: this iterator`
}

// A pointer-receiver Compare compares pointees; distinct pointers tie.
func ptrKeysMethod(m map[*PtrKey]bool) []*PtrKey {
	return slices.SortedFunc(maps.Keys(m), (*PtrKey).Compare) // want `TS-T02: this iterator`
}

// A promoted method compares the embedded field, not the whole key.
func wrappedPromotedMethod(m map[Wrapped]bool) []Wrapped {
	return slices.SortedFunc(maps.Keys(m), func(a, b Wrapped) int { return a.Compare(b.OID) }) // want `TS-T02: this iterator`
}

// Bool leaf: no comparison of a bool counts as covering it.
func flaggedKeysSkipBool(m map[Flagged]bool) []Flagged {
	return slices.SortedFunc(maps.Keys(m), func(a, b Flagged) int { return cmp.Compare(a.X, b.X) }) // want `TS-T02: this iterator`
}

// Pointer element: distinct pointers with equal pointees tie.
func pointerKeys(m map[*item]bool) []*item {
	return slices.SortedFunc(maps.Keys(m), func(a, b *item) int { // want `TS-T02: this iterator`
		return cmp.Or(strings.Compare(a.name, b.name), cmp.Compare(a.count, b.count))
	})
}

func pointerValues(m map[string]*item) []*item {
	return slices.SortedFunc(maps.Values(m), func(a, b *item) int { return strings.Compare(a.name, b.name) }) // want `TS-T02: this iterator`
}

// Pointer leaf inside a struct.
func pointerLeaf(m map[Linked]bool) []Linked {
	return slices.SortedFunc(maps.Keys(m), func(a, b Linked) int { return strings.Compare(a.Name, b.Name) }) // want `TS-T02: this iterator`
}

// Promotion through an embedded pointer is a pointer leaf.
func promotedThroughPointer(m map[PtrHost]bool) []PtrHost {
	return slices.SortedFunc(maps.Keys(m), func(a, b PtrHost) int { // want `TS-T02: this iterator`
		return cmp.Or(cmp.Compare(a.Zone, b.Zone), cmp.Compare(a.Name, b.Name))
	})
}

// Interface element and interface leaf.
func interfaceKeys(m map[Node]bool) []Node {
	return slices.SortedFunc(maps.Keys(m), func(a, b Node) int { return cmp.Compare(a.Pos(), b.Pos()) }) // want `TS-T02: this iterator`
}

func interfaceLeaf(m map[Boxed]bool) []Boxed {
	return slices.SortedFunc(maps.Keys(m), func(a, b Boxed) int { return strings.Compare(a.Name, b.Name) }) // want `TS-T02: this iterator`
}

// Float leaf, compared and all: -0 and +0 compare equal.
func floatLeaf(m map[Scored]bool) []Scored {
	return slices.SortedFunc(maps.Keys(m), func(a, b Scored) int { // want `TS-T02: this iterator`
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Score, b.Score))
	})
}

func floatValues(m map[string]float64) []float64 {
	var out []float64
	for _, v := range m { // want `TS-T02: this loop collects map entries and sorts them`
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b float64) int { return cmp.Compare(a, b) })
	return out
}

// time.Time: Compare returns 0 for one instant in two locations, and its
// location is a pointer.
func timeField(m map[Stamped]bool) []Stamped {
	return slices.SortedFunc(maps.Keys(m), func(a, b Stamped) int { // want `TS-T02: this iterator`
		return cmp.Or(a.At.Compare(b.At), strings.Compare(a.Name, b.Name))
	})
}

func timeKeys(m map[time.Time]bool) []time.Time {
	return slices.SortedFunc(maps.Keys(m), time.Time.Compare) // want `TS-T02: this iterator`
}

// Asymmetric earlier tie-break.
func asymmetricPrefix(m map[string]int) []item {
	var items []item
	for name, count := range m { // want `TS-T02: this loop collects map entries and sorts them`
		items = append(items, item{name, count})
	}
	slices.SortFunc(items, func(x, y item) int {
		return cmp.Or(cmp.Compare(x.count, y.count+1), strings.Compare(x.name, y.name), cmp.Compare(x.count, y.count))
	})
	return items
}

// Earlier tie-break that reads both arguments on one side.
func crossedPrefix(m map[string]int) []item {
	var items []item
	for name, count := range m { // want `TS-T02: this loop collects map entries and sorts them`
		items = append(items, item{name, count})
	}
	slices.SortFunc(items, func(x, y item) int {
		return cmp.Or(cmp.Compare(x.count-y.count, y.count-x.count), strings.Compare(x.name, y.name), cmp.Compare(x.count, y.count))
	})
	return items
}

// Earlier tie-break that calls a function.
func callingPrefix(m map[string]int, weight func(string) int) []string {
	var names []string
	for name := range m { // want `TS-T02: this loop collects map entries and sorts them`
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(weight(a), weight(b)), strings.Compare(a, b))
	})
	return names
}

// One argument compared with itself.
func selfCompare(m map[string]int) []string {
	return slices.SortedFunc(maps.Keys(m), func(a, b string) int { return strings.Compare(a, a) }) // want `TS-T02: this iterator`
}

// Subtraction is not a comparison: it overflows.
func subtraction(m map[int]bool) []int {
	return slices.SortedFunc(maps.Keys(m), func(a, b int) int { return a - b }) // want `TS-T02: this iterator`
}

func partialItems(m map[string]int) []item {
	var items []item
	for name, count := range m { // want `TS-T02: this loop collects map entries and sorts them`
		items = append(items, item{name, count})
	}
	slices.SortStableFunc(items, func(x, y item) int { return -cmp.Compare(x.count, y.count) })
	return items
}

// Expected false positive: a comparator held in a variable is not followed.
func namedComparator(m map[Hash]bool) []Hash {
	byBytes := func(a, b Hash) int { return bytes.Compare(a[:], b[:]) }
	return slices.SortedFunc(maps.Keys(m), byBytes) // want `TS-T02: this iterator`
}

func compareHashes(a, b Hash) int { return bytes.Compare(a[:], b[:]) }

// Expected false positive: a plain function earns no fact; only methods do.
func plainFunctionComparator(m map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(m), compareHashes) // want `TS-T02: this iterator`
}

type PtrCommit struct{ ID *OID }

// A fact-bearing method reached through a pointer field compares pointees;
// two distinct pointers to equal OIDs tie.
func pointerFieldMethod(m map[PtrCommit]bool) []PtrCommit {
	return slices.SortedFunc(maps.Keys(m), func(a, b PtrCommit) int { return a.ID.Compare(*b.ID) }) // want `TS-T02: this iterator`
}

type Blob struct {
	Name string
	Data []byte
}

// A slice leaf is not in the covered list: only whole arrays are. Equal
// contents print the same, so this is cautious rather than a known leak.
func sliceLeaf(m map[string]Blob) []Blob {
	return slices.SortedFunc(maps.Values(m), func(a, b Blob) int { // want `TS-T02: this iterator`
		return cmp.Or(cmp.Compare(a.Name, b.Name), bytes.Compare(a.Data, b.Data))
	})
}
