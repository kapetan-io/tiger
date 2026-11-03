// Package inv declares every invariant the ledger enforces.
package inv


type ID string

const (
	HeaderChecksum ID = "header-checksum"
	HeaderSize     ID = "header-size"
)
