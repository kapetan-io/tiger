// Package inv declares every invariant the ledger enforces.
package inv

// ID names one invariant. Pass it to assert.Invariant and assert.Violates.
type ID string

const (
	HeaderChecksum ID = "header-checksum"
	HeaderSize     ID = "header-size"
)
