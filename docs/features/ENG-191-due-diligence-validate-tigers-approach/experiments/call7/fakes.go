// Package call7 holds the shapes behind ENG-191 call 7: fakes that pass
// today's name-based rules, the goroutine owners TS-C02 should accept or
// reject, and the shutdown loop shapes the one-exit rule separates.
package call7

import "sync"

// Latch has a Done method that never fires, which today's TS-S03 and TS-C05
// accept because of its name.
type Latch struct{}

// Done returns a nil channel, so a receive on it blocks forever.
func (Latch) Done() <-chan bool { return nil }

// DrainLatch sums work until the latch fires, which it never does.
func DrainLatch(latch Latch, work <-chan int) int {
	total := 0
	for {
		select {
		case <-latch.Done():
			return total
		case n := <-work:
			total += n
		}
	}
}

// Worker selects on a channel named shutdown that nothing ever closes or
// sends on.
type Worker struct {
	shutdown chan bool
	work     chan int
}

// NewWorker returns a worker whose shutdown channel is never used.
func NewWorker(work chan int) *Worker {
	return &Worker{shutdown: make(chan bool), work: work}
}

// Run sums work until shutdown, which never comes.
func (w *Worker) Run() int {
	total := 0
	for {
		select {
		case <-w.shutdown:
			return total
		case n := <-w.work:
			total += n
		}
	}
}

// Buf is a pooled buffer that remembers who used it last.
type Buf struct {
	User string
	Data []byte
}

// Reset clears nothing, which today's TS-M05 accepts because of its name.
func (b *Buf) Reset() {}

var bufPool = sync.Pool{New: func() any { return new(Buf) }}

// UseWithEmptyReset fills a pooled buffer for user and puts it back after an
// empty Reset, so the next user sees it.
func UseWithEmptyReset(user string) {
	b := bufPool.Get().(*Buf)
	b.User = user
	b.Reset()
	bufPool.Put(b)
}

// UseWithZero zeroes the buffer before putting it back, the only shape call 7
// keeps for TS-M05.
func UseWithZero(user string) {
	b := bufPool.Get().(*Buf)
	b.User = user
	*b = Buf{}
	bufPool.Put(b)
}

// PeekPooled takes a buffer from the pool and reports who used it last.
func PeekPooled() string {
	b := bufPool.Get().(*Buf)
	defer bufPool.Put(b)
	return b.User
}
