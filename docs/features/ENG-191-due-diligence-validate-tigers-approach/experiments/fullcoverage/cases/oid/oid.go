// Package oid reproduces mono-repo@321da03 services/git-server/internal/storage/memory/memory.go:833
// (graphIndex.Ancestry): commits ordered by committer time descending, ties by OID
// descending. storage.OID is a struct holding a [32]byte buffer, so it is not cmp.Ordered.
package oid

import (
	"bytes"
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"fullcoverage/cases/variant"
)

type HashFormat uint8

func (f HashFormat) rawLen() int {
	if f == 1 {
		return 32
	}
	return 20
}

// OID mirrors storage.OID: comparable, map-keyable, not ordered.
type OID struct {
	format HashFormat
	raw    [32]byte
}

func (o OID) String() string { return hex.EncodeToString(o.raw[:o.format.rawLen()]) }

// Compare is the S5 addition: a total order over every field of the key.
func (o OID) Compare(p OID) int {
	return cmp.Or(cmp.Compare(o.format, p.format), bytes.Compare(o.raw[:], p.raw[:]))
}

type Object struct {
	Commit bool
	Time   int64
}

func parseTime(o Object) (int64, error) {
	if o.Time < 0 {
		return 0, errors.New("bad commit")
	}
	return o.Time, nil
}

// Original is the shipped shape: deterministic (String tie-break) but a comparator sort.
func Original(interesting map[OID]bool, objects map[OID]Object) ([]OID, error) {
	ordered := make([]OID, 0, len(interesting))
	times := map[OID]int64{}
	for oid := range interesting {
		obj, ok := objects[oid]
		if !ok || !obj.Commit {
			continue
		}
		t, err := parseTime(obj)
		if err != nil {
			return nil, err
		}
		times[oid] = t
		ordered = append(ordered, oid)
	}
	sort.Slice(ordered, func(i, j int) bool {
		ti, tj := times[ordered[i]], times[ordered[j]]
		if ti != tj {
			return ti > tj // committer-date descending
		}
		return ordered[i].String() > ordered[j].String() // deterministic tie-break
	})
	return ordered, nil
}

// Bare drops the tie-break, proving the fixture has ties.
func Bare(interesting map[OID]bool, objects map[OID]Object) ([]OID, error) {
	ordered := make([]OID, 0, len(interesting))
	times := map[OID]int64{}
	for oid := range interesting {
		obj, ok := objects[oid]
		if !ok || !obj.Commit {
			continue
		}
		t, err := parseTime(obj)
		if err != nil {
			return nil, err
		}
		times[oid] = t
		ordered = append(ordered, oid)
	}
	sort.Slice(ordered, func(i, j int) bool { return times[ordered[i]] > times[ordered[j]] })
	return ordered, nil
}

// S2 ranges over the keys pre-sorted by a key-only comparator, then a stable sort by time.
func S2(interesting map[OID]bool, objects map[OID]Object) ([]OID, error) {
	ordered := make([]OID, 0, len(interesting))
	times := map[OID]int64{}
	byHex := func(a, b OID) int { return strings.Compare(b.String(), a.String()) }
	for _, oid := range slices.SortedFunc(maps.Keys(interesting), byHex) {
		obj, ok := objects[oid]
		if !ok || !obj.Commit {
			continue
		}
		t, err := parseTime(obj)
		if err != nil {
			return nil, err
		}
		times[oid] = t
		ordered = append(ordered, oid)
	}
	slices.SortStableFunc(ordered, func(a, b OID) int { return cmp.Compare(times[b], times[a]) })
	return ordered, nil
}

// S2Keep pre-sorts the keys with a key-only comparator and keeps the original sort untouched.
func S2Keep(interesting map[OID]bool, objects map[OID]Object) ([]OID, error) {
	ordered := make([]OID, 0, len(interesting))
	times := map[OID]int64{}
	for _, oid := range slices.SortedFunc(maps.Keys(interesting), OID.Compare) {
		obj, ok := objects[oid]
		if !ok || !obj.Commit {
			continue
		}
		t, err := parseTime(obj)
		if err != nil {
			return nil, err
		}
		times[oid] = t
		ordered = append(ordered, oid)
	}
	sort.Slice(ordered, func(i, j int) bool {
		ti, tj := times[ordered[i]], times[ordered[j]]
		if ti != tj {
			return ti > tj // committer-date descending
		}
		return ordered[i].String() > ordered[j].String() // deterministic tie-break
	})
	return ordered, nil
}

// S2Partial is the unsound key-only comparator: it reads only a and b but compares one byte.
func S2Partial(interesting map[OID]bool, objects map[OID]Object) ([]OID, error) {
	ordered := make([]OID, 0, len(interesting))
	times := map[OID]int64{}
	firstByte := func(a, b OID) int { return cmp.Compare(a.raw[0], b.raw[0]) }
	for _, oid := range slices.SortedFunc(maps.Keys(interesting), firstByte) {
		obj, ok := objects[oid]
		if !ok || !obj.Commit {
			continue
		}
		t, err := parseTime(obj)
		if err != nil {
			return nil, err
		}
		times[oid] = t
		ordered = append(ordered, oid)
	}
	slices.SortStableFunc(ordered, func(a, b OID) int { return cmp.Compare(times[b], times[a]) })
	return ordered, nil
}

// S3 collects from the map, sorts by key, then stable-sorts by time. Both sorts take comparators.
func S3(interesting map[OID]bool, objects map[OID]Object) ([]OID, error) {
	ordered := make([]OID, 0, len(interesting))
	times := map[OID]int64{}
	for oid := range interesting {
		obj, ok := objects[oid]
		if !ok || !obj.Commit {
			continue
		}
		t, err := parseTime(obj)
		if err != nil {
			return nil, err
		}
		times[oid] = t
		ordered = append(ordered, oid)
	}
	slices.SortFunc(ordered, func(a, b OID) int { return b.Compare(a) })
	slices.SortStableFunc(ordered, func(a, b OID) int { return cmp.Compare(times[b], times[a]) })
	return ordered, nil
}

// S4 makes the one comparator total with cmp.Or ending on the key.
func S4(interesting map[OID]bool, objects map[OID]Object) ([]OID, error) {
	ordered := make([]OID, 0, len(interesting))
	times := map[OID]int64{}
	for oid := range interesting {
		obj, ok := objects[oid]
		if !ok || !obj.Commit {
			continue
		}
		t, err := parseTime(obj)
		if err != nil {
			return nil, err
		}
		times[oid] = t
		ordered = append(ordered, oid)
	}
	slices.SortFunc(ordered, func(a, b OID) int {
		return cmp.Or(cmp.Compare(times[b], times[a]), b.Compare(a))
	})
	return ordered, nil
}

// S6Hex collects the hex form (an ordered string) and sorts it with sort.Strings, then
// parses back. The collect loop must be pure, so the commit parse moves after the sort.
func S6Hex(interesting map[OID]bool, objects map[OID]Object) ([]OID, error) {
	var hexes []string
	byHex := map[string]OID{}
	for oid := range interesting {
		h := oid.String()
		byHex[h] = oid
		hexes = append(hexes, h)
	}
	sort.Strings(hexes)
	ordered := make([]OID, 0, len(hexes))
	times := map[OID]int64{}
	for _, h := range slices.Backward(hexes) {
		oid := byHex[h]
		obj, ok := objects[oid]
		if !ok || !obj.Commit {
			continue
		}
		t, err := parseTime(obj)
		if err != nil {
			return nil, err
		}
		times[oid] = t
		ordered = append(ordered, oid)
	}
	slices.SortStableFunc(ordered, func(a, b OID) int { return cmp.Compare(times[b], times[a]) })
	return ordered, nil
}

func fixture() (map[OID]bool, map[OID]Object) {
	interesting := map[OID]bool{}
	objects := map[OID]Object{}
	for i := range 12 {
		var o OID
		o.raw[0] = byte(i % 3) // S2Partial ties on the first byte
		o.raw[1] = byte(i)
		interesting[o] = true
		objects[o] = Object{Commit: i != 5, Time: int64(100 + i/4)} // 4 commits per timestamp
	}
	return interesting, objects
}

func render(f func(map[OID]bool, map[OID]Object) ([]OID, error)) func() string {
	return func() string {
		out, err := f(fixture())
		return fmt.Sprint(out, err)
	}
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "Bare", Run: render(Bare)},
	{Name: "S2", Run: render(S2)},
	{Name: "S2Keep", Run: render(S2Keep)},
	{Name: "S2Partial", Run: render(S2Partial)},
	{Name: "S3", Run: render(S3)},
	{Name: "S4", Run: render(S4)},
	{Name: "S6Hex", Run: render(S6Hex)},
}
