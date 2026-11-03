// Package assertpkg recognizes tiger's assert package by import path. A
// package that merely calls itself assert gets none of the exemptions and
// none of the assertion credit.
package assertpkg

const Path = "github.com/kapetan-io/tiger/assert"

func Is(path string) bool {
	return path == Path || path == Path+"_test"
}
