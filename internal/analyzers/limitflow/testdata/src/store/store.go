package store // want package:`\[types.ListOptions.Limit\]`

import "types"

// Rows is a cursor.
type Rows interface{ Next() bool }

// Lister lists rows.
type Lister interface {
	List(rows Rows, opts types.ListOptions) int
}

// Memory lists from memory.
type Memory struct{}

// List counts at most opts.Limit rows.
func (Memory) List(rows Rows, opts types.ListOptions) int { // want List:`\[\]`
	count := 0
	for ; count < opts.Limit && rows.Next(); count++ {
	}
	return count
}
