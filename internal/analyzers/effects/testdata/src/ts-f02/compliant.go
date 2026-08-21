// The TS-F02 compliant shapes: a pinned caller of a pinned callee checks
// modularly from the callee's signature alone, and a pinned function whose
// unexported helpers stay within the pin is silent too.
package ts02

// Inner is pinned to allocate.
//
//tiger:effects alloc
func Inner() []int { // want Inner:`alloc`
	return make([]int, 2)
}

// Outer calls Inner and matches its pin exactly — modular checking passes
// from Inner's pin alone, without looking beneath it.
//
//tiger:effects alloc
func Outer() []int { // want Outer:`alloc`
	return Inner()
}

// helper does nothing effectful.
func helper() int {
	return 3
}

// Wrapped calls only its unexported helper, which stays within the pin.
//
//tiger:effects none
func Wrapped() int { // want Wrapped:`none`
	return helper()
}
