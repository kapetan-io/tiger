// Package broken fails to type-check, so the run must exit 2.
package broken

// Mismatched returns a string where an int is declared.
func Mismatched() int {
	return "not an int"
}
