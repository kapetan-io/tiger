package fixture

const backoffMin = 1

const _ = uint(backoffMin - 1)

// firstBackoff repeats the blank spec's expression, so backoffMin is used.
const (
	_ = backoffMin
	firstBackoff
)

func backoffs() int {
	return firstBackoff
}
