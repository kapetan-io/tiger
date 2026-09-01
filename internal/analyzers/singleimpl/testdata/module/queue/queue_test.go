package queue

// fakeQueue is a test double: TS-X01 never counts a type declared in a
// _test.go file as an implementation, so this second implementation must
// not clear the finding on Queue.
type fakeQueue struct {
	items []int
}

// Push appends v.
func (f *fakeQueue) Push(v int) {
	f.items = append(f.items, v)
}

// Pop removes and returns the oldest item.
func (f *fakeQueue) Pop() (int, bool) {
	if len(f.items) == 0 {
		return 0, false
	}
	v := f.items[0]
	f.items = f.items[1:]
	return v, true
}
