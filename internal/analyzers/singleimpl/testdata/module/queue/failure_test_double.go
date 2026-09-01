// Package queue has one production implementation of Queue; a second
// implementation lives only in a _test.go file and must not count.
package queue

// Queue has exactly one implementation outside _test.go files.
type Queue interface { // want `interface Queue has one implementation, ringQueue`
	Push(v int)
	Pop() (int, bool)
}

// ringQueue is Queue's only production implementation.
type ringQueue struct {
	items []int
}

// Push appends v.
func (r *ringQueue) Push(v int) {
	r.items = append(r.items, v)
}

// Pop removes and returns the oldest item.
func (r *ringQueue) Pop() (int, bool) {
	if len(r.items) == 0 {
		return 0, false
	}
	v := r.items[0]
	r.items = r.items[1:]
	return v, true
}
