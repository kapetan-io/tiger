package gaps_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gaps"
)

// The reported worklist walks a tree completely.
func TestWalkReportedEndsOnATree(t *testing.T) {
	leaf1, leaf2 := &gaps.Node{}, &gaps.Node{}
	visited, err := gaps.WalkReported(&gaps.Node{Next: []*gaps.Node{leaf1, leaf2}})
	require.NoError(t, err)
	assert.Equal(t, 3, visited)
}

// The reported worklist fails loudly on a cycle instead of returning a partial walk.
func TestWalkReportedFailsOnACycle(t *testing.T) {
	a, b := &gaps.Node{}, &gaps.Node{}
	a.Next = []*gaps.Node{b}
	b.Next = []*gaps.Node{a}
	_, err := gaps.WalkReported(a)
	require.ErrorContains(t, err, "walk reached nodesMax")
}
