// Package core pins an exported function whose effect violation lives in
// the helper package: TS-F02's sparse-pins, dense-enforcement contract
// across a package boundary.
package core

import "fixture.example/facts/helper"

// Notify claims purity but reaches the network through the helper.
//
//tiger:effects none
func Notify() error {
	return helper.Ping()
}
