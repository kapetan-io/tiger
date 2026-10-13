package gaps_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gaps"
)

// Every loop here runs forever and gets no TS-V01 finding. TS-S02 blocks only
// Overflow, and TS-S09 flags LabeledContinue and GotoLoop as style.
func TestLoopsTigerAcceptsRunForever(t *testing.T) {
	for _, test := range []struct {
		name string
		loop func()
	}{
		{name: "ShadowSlice", loop: func() { gaps.ShadowSlice([]byte{1}) }},
		{name: "AliasAppend", loop: func() { gaps.AliasAppend([]int{1}) }},
		{name: "ClosureGrow", loop: func() { gaps.ClosureGrow([]int{1}) }},
		{name: "LabeledContinue", loop: func() { gaps.LabeledContinue([]int{1}) }},
		{name: "Overflow", loop: gaps.Overflow},
		{name: "WrongCounter", loop: func() { gaps.WrongCounter(1) }},
		{name: "CounterUndone", loop: func() { gaps.CounterUndone(5) }},
		{name: "LimitGrows", loop: func() { gaps.LimitGrows(1) }},
		{name: "HelperGrow", loop: gaps.HelperGrow},
		{name: "MapGrow", loop: func() { gaps.MapGrow(map[int]int{0: 0}) }},
		{name: "RangeNaturals", loop: gaps.RangeNaturals},
		{name: "GotoLoop", loop: gaps.GotoLoop},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.False(t, finishes(test.loop))
		})
	}
}
