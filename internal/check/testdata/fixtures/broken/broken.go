// Package broken fails to type-check, so the run must exit 2.
package broken


func Mismatched() int {
	return "not an int"
}
