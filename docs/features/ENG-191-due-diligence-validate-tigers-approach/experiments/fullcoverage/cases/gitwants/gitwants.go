// Package gitwants reproduces github.com/go-git/go-git/v5@v5.19.1 remote.go:1081
// (getWants): collect the keys of a map[plumbing.Hash]bool, where plumbing.Hash is
// [20]byte. The order is fixed only later, at encode time, by plumbing.HashesSort
// (sort.Sort with bytes.Compare over the whole hash).
package gitwants

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"sort"

	"fullcoverage/cases/variant"
)

type Hash [20]byte

// Compare is the S5 addition: a total order over the whole key.
func (h Hash) Compare(o Hash) int { return bytes.Compare(h[:], o[:]) }

type HashSlice []Hash

func (p HashSlice) Len() int           { return len(p) }
func (p HashSlice) Less(i, j int) bool { return bytes.Compare(p[i][:], p[j][:]) < 0 }
func (p HashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// HashesSort mirrors plumbing.HashesSort.
func HashesSort(a []Hash) { sort.Sort(HashSlice(a)) }

// Original is the shipped shape: getWants returns map order.
func Original(wants map[Hash]bool) []Hash {
	var result []Hash
	for h := range wants {
		result = append(result, h)
	}

	return result
}

// Sorted is Original plus the encode-time sort, placed directly after the loop.
func Sorted(wants map[Hash]bool) []Hash {
	var result []Hash
	for h := range wants {
		result = append(result, h)
	}
	HashesSort(result)
	return result
}

// S2 returns the keys sorted by a key-only comparator over the whole array.
func S2(wants map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(wants), func(a, b Hash) int { return bytes.Compare(a[:], b[:]) })
}

// S5 uses a Compare method declared on the key type.
func S5(wants map[Hash]bool) []Hash {
	return slices.SortedFunc(maps.Keys(wants), Hash.Compare)
}

// S3 collects and immediately sorts with the key-only comparator: same exactness as S2,
// but the collect loop is a map range followed by a comparator sort, so it fires.
func S3(wants map[Hash]bool) []Hash {
	var result []Hash
	for h := range wants {
		result = append(result, h)
	}
	slices.SortFunc(result, func(a, b Hash) int { return bytes.Compare(a[:], b[:]) })
	return result
}

// S6 collects string(h[:]) so sort.Strings applies; the conversion is a call, so the
// collect loop is not pure under the prototype.
func S6(wants map[Hash]bool) []Hash {
	var keys []string
	for h := range wants {
		keys = append(keys, string(h[:]))
	}
	sort.Strings(keys)
	result := make([]Hash, len(keys))
	for i, k := range keys {
		copy(result[i][:], k)
	}
	return result
}

func fixture() map[Hash]bool {
	m := map[Hash]bool{}
	for i := range 10 {
		var h Hash
		h[0], h[19] = byte(i%3), byte(i)
		m[h] = true
	}
	return m
}

func render(f func(map[Hash]bool) []Hash) func() string {
	return func() string { return fmt.Sprintf("%x", f(fixture())) }
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "Sorted", Run: render(Sorted)},
	{Name: "S2", Run: render(S2)},
	{Name: "S3", Run: render(S3)},
	{Name: "S5", Run: render(S5)},
	{Name: "S6", Run: render(S6)},
}
