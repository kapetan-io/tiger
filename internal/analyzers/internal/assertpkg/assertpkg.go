// Package assertpkg recognizes tiger's assert package by import path. A
// package that merely calls itself assert gets none of the exemptions and
// none of the assertion credit.
package assertpkg

// Path is the import path of tiger's assert package.
const Path = "github.com/kapetan-io/tiger/assert"

// Is reports whether path names tiger's assert package or its external test
// package.
func Is(path string) bool {
	return path == Path || path == Path+"_test"
}
