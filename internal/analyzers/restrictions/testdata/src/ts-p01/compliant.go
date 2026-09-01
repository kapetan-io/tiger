// want package:`no-reflect, imports\(fixture\.example/ts-p01helper/domain/\.\.\.\)`
package tsp01

import "fixture.example/ts-p01helper/domain"

// UseDomain imports only the path the declared imports(...) subtree
// allows, plus the standard library, which is always allowed — no finding
// on either import.
func UseDomain() string {
	return domain.Marker
}
