// Package outside imports a standard-library package so a finish step can
// try to position a finding in a file the driver never loaded.
package outside

import "fmt"

// Say prints its argument.
func Say(text string) {
	fmt.Println(text)
}
