package fixture

// chainMax bounds how many tags one lookup follows.
const chainMax = 16

// nestedMin exists only so chainMax has something to be compared with.
const nestedMin = 2 // want `TS-S21: nestedMin is a limit that only appears inside a relational assertion`

const _ = uint(chainMax - nestedMin)

func followTags() int {
	n := 0
	for i := 0; i < chainMax; i++ {
		n++
	}
	return n
}
