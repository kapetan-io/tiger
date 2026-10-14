package cappedworklist_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cappedworklist "github.com/kapetan-io/tiger/patterns/capped-worklist"
)

func cycle() *cappedworklist.Node {
	first, second := &cappedworklist.Node{}, &cappedworklist.Node{}
	first.Next = []*cappedworklist.Node{second}
	second.Next = []*cappedworklist.Node{first}
	return first
}

func star(leaves int) *cappedworklist.Node {
	root := &cappedworklist.Node{}
	for range leaves {
		root.Next = append(root.Next, &cappedworklist.Node{})
	}
	return root
}

// TestWalkCountsTree walks graphs below, at and above the cap.
//
// Goal: Walk returns the full count without error up to nodesMax nodes, and
// ErrWalkTooLarge once the graph is larger.
func TestWalkCountsTree(t *testing.T) {
	for _, test := range []struct {
		name    string
		root    *cappedworklist.Node
		want    int
		wantErr error
	}{
		{name: "BelowCap", root: star(2), want: 3},
		{name: "ExactlyAtCap", root: star(99), want: 100},
		{name: "OneOverCap", root: star(100), wantErr: cappedworklist.ErrWalkTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			count, err := cappedworklist.Walk(test.root)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, count)
		})
	}
}

// TestWalkReportsCycle walks a two-node cycle.
//
// Goal: the cap fires as ErrWalkTooLarge, so the caller learns the walk never ended.
func TestWalkReportsCycle(t *testing.T) {
	_, err := cappedworklist.Walk(cycle())
	require.ErrorIs(t, err, cappedworklist.ErrWalkTooLarge)
}

// TestReadRefsTruncatedHidesLongInput reads five refs with a limit of two.
//
// Goal: the broken shape returns two refs and no signal, so the caller cannot
// tell a cut-short read from a complete one.
func TestReadRefsTruncatedHidesLongInput(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("a\nb\nc\nd\ne\n"))
	assert.Equal(t, []string{"a", "b"}, cappedworklist.ReadRefsTruncated(scanner, 2))
}
