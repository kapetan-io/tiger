// Package rpcclient stands in for a third-party gRPC client: its calls do
// IO, but it is not on the standard library IO list, so ioinloop reports
// them only when a repo names its package in ioinloop.packages.
package rpcclient

// Call sends one request to the remote service.
func Call(id string) error {
	return nil
}
