package loopfixes

import (
	"errors"
	"maps"
	"slices"
)

const nodesMax = 1000

const _ = uint(1<<20 - nodesMax)

var errTooLarge = errors.New("worklist reached nodesMax")

// HelperGrow caps the worklist and reports the cap.
func HelperGrow() error {
	stack := []int{0}
	i := 0
	for ; i < len(stack) && i < nodesMax; i++ {
		stack = append(stack, i)
	}
	if i == nodesMax && i < len(stack) {
		return errTooLarge
	}
	return nil
}

// MapGrow caps the walk over the map's keys and reports the cap.
func MapGrow(entries map[int]int) error {
	keys := slices.Sorted(maps.Keys(entries))
	i := 0
	for ; i < len(keys) && i < nodesMax; i++ {
		entries[keys[i]]++
	}
	if i == nodesMax && i < len(keys) {
		return errTooLarge
	}
	return nil
}
