package store // want package:`\[types.ListOptions.Limit\]`

import "types"

type Rows interface{ Next() bool }

type Lister interface {
	List(rows Rows, opts types.ListOptions) int
}

type Memory struct{}

func (Memory) List(rows Rows, opts types.ListOptions) int {
	count := 0
	for ; count < opts.Limit && rows.Next(); count++ {
	}
	return count
}
