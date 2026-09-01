package app // want `probe: finished fixture\.example/matching/app`

import "fixture.example/matching/dep" // want "probe: import"

// Use calls Helper.
func Use() int {
	return dep.Helper()
}
