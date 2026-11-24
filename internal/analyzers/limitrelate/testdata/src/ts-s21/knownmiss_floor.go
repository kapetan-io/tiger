package fixture

// itemMax is the real limit and fill uses it. itemFloor and copyMin were
// invented for the guards.
// known-miss: itemFloor does not end in Max or Min, so TS-S21 never
// considers it a limit. copyMin is mentioned by unusedCopy, a constant
// nothing reads, and the analyzer counts that mention as a use.
const itemMax = 64
const itemFloor = 1
const copyMin = 2

const _ = uint(itemMax - itemFloor)
const _ = uint(itemMax - copyMin)

const unusedCopy = copyMin

func fill() int {
	return itemMax
}
