package fixture

// Each limit below is mentioned outside its guard only by a blank binding,
// which reads the name and does nothing with it.
const blankVarMin = 1    // want `TS-S21: blankVarMin is a limit that only appears inside a relational assertion`
const blankConstMin = 1  // want `TS-S21: blankConstMin is a limit that only appears inside a relational assertion`
const blankAssignMin = 1 // want `TS-S21: blankAssignMin is a limit that only appears inside a relational assertion`

const _ = uint(blankVarMin)
const _ = uint(blankConstMin)
const _ = uint(blankAssignMin)

var _ = blankVarMin

const _ = blankConstMin

func discard() {
	_ = blankAssignMin
}
