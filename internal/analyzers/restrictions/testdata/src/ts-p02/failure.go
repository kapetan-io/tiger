// Package tsp02 claims every axis but imports two dependencies that leave
// axes unclaimed: ts02helper claims no-reflect only, weakening
// closed-dispatch and imports(...); ts02bare declares nothing, weakening
// every axis. ts02bare sorts before ts02helper, so it names every line.
//
// want +3 `TS-P02: package tsp02 claims closed-dispatch but imports fixture\.example/ts-p02bare, which declares nothing — add //tiger:restrict closed-dispatch to fixture\.example/ts-p02bare, or drop closed-dispatch from tsp02's declaration` `TS-P02: package tsp02 claims no-reflect but imports fixture\.example/ts-p02bare, which declares nothing` `TS-P02: package tsp02 claims imports\(\.\.\.\) but imports fixture\.example/ts-p02bare, which declares nothing`
//
//tiger:restrict closed-dispatch, no-reflect, imports(fixture.example/ts-p02bare, fixture.example/ts-p02helper)
package tsp02

import (
	"fixture.example/ts-p02bare"
	"fixture.example/ts-p02helper"
)

// Use references both dependencies so the imports above are used.
func Use() string {
	return ts02helper.Marker + ts02bare.Marker
}
