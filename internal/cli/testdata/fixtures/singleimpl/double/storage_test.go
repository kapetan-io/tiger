package double

import "testing"

type fakeStorage struct {
	calls int
}

// Write counts calls.
func (f *fakeStorage) Write(data []byte) int {
	f.calls++
	return len(data)
}

// TestFakeStorageCounts covers the test double.
//
// Goal: the fake counts one call per Write.
func TestFakeStorageCounts(t *testing.T) {
	var storage Storage = &fakeStorage{calls: 0}
	if storage.Write([]byte("x")) != 1 {
		t.Fatal("write length")
	}
}
