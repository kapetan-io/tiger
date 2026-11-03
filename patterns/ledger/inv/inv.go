// Package inv declares every invariant the ledger example enforces.
//
// One package, one declaration per invariant, referenced from every assertion
// site (TS-A07). The analyzer counts the functions referencing each ID; the
// string is what a failed assertion prints.
package inv

type ID string

const (
	HeaderChecksum ID = "header-checksum"
	HeaderSize     ID = "header-size"
)
