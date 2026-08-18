// The TS-T02 failure modes: every shape whose body lets map iteration order
// reach an output, plus one loop that is genuinely order-safe but sits
// outside the closed allowlist maporder proves against.
package fixture

import (
	"fmt"
	"io"
)

// collectValuesAppends appends every value to a slice while ranging over m,
// so the slice's element order mirrors map iteration order.
func collectValuesAppends(m map[string]int) []int {
	var result []int
	for _, v := range m { // want `TS-T02: ranging over a map appends to a slice`
		result = append(result, v)
	}
	return result
}

// printValuesCalls writes every entry to w while ranging over m, so the
// printed order mirrors map iteration order.
func printValuesCalls(m map[string]int, w io.Writer) {
	for k, v := range m { // want `TS-T02: ranging over a map calls a function`
		fmt.Fprintf(w, "%s=%d\n", k, v)
	}
}

// joinKeysBuilds concatenates every key into a string while ranging over m,
// so the joined order mirrors map iteration order.
func joinKeysBuilds(m map[string]int) string {
	result := ""
	for k := range m { // want `TS-T02: ranging over a map builds a string`
		result = result + k
	}
	return result
}

// forwardValuesSends sends every value on out while ranging over m, so the
// order values arrive on the channel mirrors map iteration order.
func forwardValuesSends(m map[string]int, out chan<- int) {
	for _, v := range m { // want `TS-T02: ranging over a map sends on a channel`
		out <- v
	}
}

// validateEachCalls calls validate with each value while ranging over m;
// validate's argument is derived from the iteration, so its call order
// mirrors map iteration order even though the loop itself builds nothing.
func validateEachCalls(m map[string]int, validate func(int) bool) {
	for _, v := range m { // want `TS-T02: ranging over a map calls a function`
		validate(v)
	}
}

// collectPositiveAppends only appends values greater than zero, wrapping the
// append in an if — still order dependent, since the surviving nested-if
// wrapper does not change what the inner append does to the result's order.
func collectPositiveAppends(m map[string]int) []int {
	var result []int
	for _, v := range m { // want `TS-T02: ranging over a map appends to a slice`
		if v > 0 {
			result = append(result, v)
		}
	}
	return result
}

// productMulKnownSafe multiplies every value into product using *=, which is
// commutative and associative exactly like the allowed += case. But the
// closed allowlist only proves order-insensitivity for +=, -=, ++, --, and
// min/max — not *=. Extending it to *= is a deliberate analyzer change with
// its own corpus case, never a configuration flag, so this genuinely safe
// loop still fires: a small amount of sorted-iteration busywork traded for
// zero false-positive risk on this blocking rule.
func productMulKnownSafe(m map[string]int) int {
	product := 1
	for _, v := range m { // want `TS-T02: this map range's body is not in the closed order-insensitive allowlist`
		product *= v
	}
	return product
}
