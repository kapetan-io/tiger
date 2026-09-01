// Package clean imports only the standard library and declares nothing,
// so no restriction is checked.
package clean

import "strings"

// Upper uppercases text.
func Upper(text string) string {
	return strings.ToUpper(text)
}
