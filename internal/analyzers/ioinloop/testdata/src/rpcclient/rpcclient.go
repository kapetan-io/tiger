// Package rpcclient stands in for a third-party gRPC client: from
// ioinloop's package-identity classifier's perspective it is exactly like
// database/sql or net/http, just not on the seed allowlist.
package rpcclient

// Call sends one request to the remote service.
func Call(id string) error {
	return nil
}
