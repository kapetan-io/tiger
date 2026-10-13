// The total-compare fact crosses packages; unexported fields from another
// package can only be covered through a method that has the fact.
package fixture

import (
	"cmp"
	"maps"
	"slices"

	"keys"
)

type Tagged struct {
	Key keys.ID
	Tag int
}

func idKeysImportedMethodExpr(m map[keys.ID]bool) []keys.ID {
	return slices.SortedFunc(maps.Keys(m), keys.ID.Compare)
}

func opaqueKeysAnyMethodName(m map[keys.Opaque]bool) []keys.Opaque {
	return slices.SortedFunc(maps.Keys(m), func(a, b keys.Opaque) int { return a.Order(b) })
}

func taggedImportedMethodOnField(m map[Tagged]bool) []Tagged {
	return slices.SortedFunc(maps.Keys(m), func(a, b Tagged) int {
		return cmp.Or(a.Key.Compare(b.Key), cmp.Compare(a.Tag, b.Tag))
	})
}

// Unexported field out of reach.
func opaqueKeysExportedOnly(m map[keys.Opaque]bool) []keys.Opaque {
	return slices.SortedFunc(maps.Keys(m), func(a, b keys.Opaque) int { return cmp.Compare(a.Name, b.Name) }) // want `TS-T02: this iterator`
}

// An imported Compare without the fact.
func partialKeysImported(m map[keys.Partial]bool) []keys.Partial {
	return slices.SortedFunc(maps.Keys(m), keys.Partial.Compare) // want `TS-T02: this iterator`
}
