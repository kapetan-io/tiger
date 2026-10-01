package gaps

import "math"

// ShadowSlice shrinks a copy that shadows data, so data never changes. TS-V01
// matches the shrink by name and accepts it.
func ShadowSlice(data []byte) {
	for len(data) > 0 {
		data := data
		data = data[1:]
		_ = data
	}
}

// AliasAppend grows data through a pointer taken before the loop. TS-V01 looks
// for &data only inside the body.
func AliasAppend(data []int) {
	alias := &data
	for len(data) > 0 {
		*alias = append(*alias, 1)
		data = data[1:]
	}
}

// ClosureGrow grows data through a closure made before the loop. TS-V01 checks
// only closures written inside the body.
func ClosureGrow(data []int) {
	grow := func() { data = append(data, 1) }
	for len(data) > 0 {
		grow()
		data = data[1:]
	}
}

// LabeledContinue skips the shrink with a continue from an inner loop. TS-V01's
// continue check stops at the inner loop.
func LabeledContinue(data []int) {
outer:
	for len(data) > 0 {
		for j := 0; j < 1; j++ {
			continue outer
		}
		data = data[1:]
	}
}

// Overflow never ends because i wraps past math.MaxInt. TS-V01 verifies n - i;
// only TS-S02 blocks it, and call 2's change 2 would let it pass.
func Overflow() {
	n := math.MaxInt
	i := 0
	for i <= n {
		i++
	}
}
