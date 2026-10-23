// Package storage stands in for a third-party client library: from
// ioinloop's package-identity classifier's perspective it is exactly like
// database/sql or net/http, just not on the seed allowlist by default.
package storage

// Save writes one item to the backing store.
func Save(item string) error {
	return nil
}
