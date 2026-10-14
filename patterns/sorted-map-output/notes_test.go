package sortedmapoutput_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sortedmapoutput "github.com/kapetan-io/tiger/patterns/sorted-map-output"
)

const runs = 200

// distinct counts the different outputs render produces over many runs.
func distinct(render func() string) int {
	seen := map[string]bool{}
	for range runs {
		seen[render()] = true
	}
	return len(seen)
}

// TestDocumentRenderIsStable attaches a statement note and a note on the call
// it wraps, which share a source span.
//
// Goal: Render prints both notes in attach order on every run.
func TestDocumentRenderIsStable(t *testing.T) {
	render := func() string {
		document := sortedmapoutput.NewDocument()
		document.Attach(&sortedmapoutput.Statement{Start: 10, Stop: 14}, "stmt")
		document.Attach(&sortedmapoutput.Call{Start: 10, Stop: 14}, "call")
		document.Attach(&sortedmapoutput.Call{Start: 20, Stop: 24}, "later")
		return document.Render()
	}
	assert.Equal(t, 1, distinct(render))
	assert.Equal(t, "10-14: stmt\n10-14: call\n20-24: later\n", render())
}

// TestIndexByPositionLosesANote indexes the same statement and call by
// position.
//
// Goal: the broken shape keeps one of the two colliding notes, picked by map
// order, so its output takes more than one value across runs.
func TestIndexByPositionLosesANote(t *testing.T) {
	render := func() string {
		return sortedmapoutput.RenderByPosition(sortedmapoutput.Notes{
			&sortedmapoutput.Statement{Start: 10, Stop: 14}: "stmt",
			&sortedmapoutput.Call{Start: 10, Stop: 14}:      "call",
			&sortedmapoutput.Call{Start: 20, Stop: 24}:      "later",
		})
	}
	assert.Equal(t, 2, distinct(render))
	require.Len(t, sortedmapoutput.IndexByPosition(sortedmapoutput.Notes{
		&sortedmapoutput.Statement{Start: 10, Stop: 14}: "stmt",
		&sortedmapoutput.Call{Start: 10, Stop: 14}:      "call",
	}), 1)
}
