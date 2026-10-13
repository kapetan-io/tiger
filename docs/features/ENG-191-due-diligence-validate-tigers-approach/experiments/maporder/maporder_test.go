package maporder_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"maporder"
)

const runs = 200

// outputs calls f runs times and counts how many different results it prints.
func outputs(f func() any) int {
	seen := map[string]bool{}
	for range runs {
		seen[fmt.Sprint(f())] = true
	}
	return len(seen)
}

func users() map[string]int {
	return map[string]int{"ann": 1, "bob": 2, "cat": 3, "dan": 4, "eve": 5, "fay": 6, "gus": 7, "hal": 8}
}

// TS-T02 blocks SortedIDs today, though its output never varies.
func TestSortedIDsNeverVaries(t *testing.T) {
	assert.Equal(t, 1, outputs(func() any { return maporder.SortedIDs(users()) }))
}

// TS-T02 blocks UnsortedIDs today and after call 8.
func TestUnsortedIDsVaries(t *testing.T) {
	assert.Greater(t, outputs(func() any { return maporder.UnsortedIDs(users()) }), 1)
}

// TS-T02 passes FirstThree today.
func TestFirstThreeVaries(t *testing.T) {
	assert.Greater(t, outputs(func() any { return maporder.FirstThree(users()) }), 1)
}

// TS-T02 never sees RankByValue.
func TestRankByValueVaries(t *testing.T) {
	ranks := map[string]int{"ann": 1, "bob": 1, "cat": 1, "dan": 2, "eve": 2, "fay": 2}
	assert.Greater(t, outputs(func() any { return maporder.RankByValue(ranks) }), 1)
}

// Accepting sort.Float64s in the collect-then-sort shape would pass this.
func TestSortedValuesVaries(t *testing.T) {
	negative := math.Copysign(0, -1)
	values := map[string]float64{"a": negative, "b": 0, "c": negative, "d": 0, "e": negative, "f": 0}
	assert.Greater(t, outputs(func() any { return maporder.SortedValues(values) }), 1)
}
