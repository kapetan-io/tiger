// Package store claims no-reflect and nothing else, so it weakens any
// importer's closed-dispatch claim.
//
//tiger:restrict no-reflect
package store

// Put stores a byte count.
func Put(data []byte) int {
	return len(data)
}
