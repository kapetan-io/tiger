package app

import "fixture.example/probe/dep"

// Use calls Helper so package app imports package dep.
func Use() int {
	return dep.Helper()
}
