// Package storage stands in for a third-party client library: its calls do
// IO, but it is not on the standard library IO list, so ioinloop reports
// them only when its package is named on -ioinloop.packages.
package storage


func Save(item string) error {
	return nil
}
