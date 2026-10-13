// Package types declares a struct whose field bounds a loop elsewhere.
package types

// ListOptions carries a page size into a store.
type ListOptions struct {
	Pivot string
	Limit int
}
