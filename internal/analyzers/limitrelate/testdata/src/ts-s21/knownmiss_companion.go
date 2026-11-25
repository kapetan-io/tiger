package fixture

// slotMax is the real limit. slotMin was invented to give slotMax something
// to compare against.
// known-miss: slotMin appears in the guard and in countSlots, so the
// guard-only check finds a use and stays silent. Whether that use is real
// is not something the analyzer can judge.
const slotMax = 32
const slotMin = 4

const _ = uint(slotMax - slotMin)

func countSlots(free int) int {
	if free < slotMin {
		return 0
	}
	return min(free, slotMax)
}
