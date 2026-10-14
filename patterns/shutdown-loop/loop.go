package shutdownloop

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// ErrClosed is returned to callers once the loop has finished.
var ErrClosed = errors.New("shutdownloop: loop closed")

// Request is one unit of work and the channel its result comes back on.
type Request struct {
	Work  func() int
	Reply chan int
}

// Loop is the right shape. The loop's only exit is the stop case, cleanup runs
// before done closes, and ctx bounds only the caller's wait.
type Loop struct {
	requests chan Request
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	cleaned  atomic.Bool
	wg       sync.WaitGroup
}

// NewLoop starts the loop.
func NewLoop() *Loop {
	loop := &Loop{
		requests: make(chan Request, 8),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	loop.wg.Go(loop.run)
	return loop
}

func (l *Loop) run() {
	// Cleanup must come before close(done): a caller released by done may read the cleaned state.
	l.serve()
	l.cleaned.Store(true)
	close(l.done)
}

func (l *Loop) serve() {
	for {
		select {
		case request := <-l.requests:
			reply(request)
		case <-l.stop:
			return
		}
	}
}

// Call runs work on the loop and returns its result.
func (l *Loop) Call(ctx context.Context, work func() int) (int, error) {
	return call(ctx, l.requests, l.done, work)
}

// Shutdown asks the loop to stop and waits for it. The context bounds this wait
// only, so a caller whose context expired can call Shutdown again. An expired
// context is reported before the wait, so the result never depends on whether
// the loop finished first.
func (l *Loop) Shutdown(ctx context.Context) error {
	l.stopOnce.Do(func() { close(l.stop) })
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-l.done:
		l.wg.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cleaned reports whether cleanup ran.
func (l *Loop) Cleaned() bool { return l.cleaned.Load() }

// EarlyExitLoop is the broken shape behind querator ENG-160. It shuts down by a
// flag, so the request case can return before the cleanup in the stop case runs,
// and done closes either way.
type EarlyExitLoop struct {
	requests   chan Request
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
	inShutdown atomic.Bool
	cleaned    atomic.Bool
	wg         sync.WaitGroup
}

// NewEarlyExitLoop starts the loop.
func NewEarlyExitLoop() *EarlyExitLoop {
	loop := &EarlyExitLoop{
		requests: make(chan Request, 8),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	loop.wg.Go(loop.run)
	return loop
}

func (l *EarlyExitLoop) run() {
	defer close(l.done)
	for {
		select {
		case request := <-l.requests:
			reply(request)
			if l.inShutdown.Load() {
				return
			}
		case <-l.stop:
			l.cleaned.Store(true)
			return
		}
	}
}

// Shutdown reports success once done closes, cleanup or not.
func (l *EarlyExitLoop) Shutdown(ctx context.Context) error {
	l.inShutdown.Store(true)
	l.stopOnce.Do(func() { close(l.stop) })
	select {
	case <-l.done:
		l.wg.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Call runs work on the loop and returns its result.
func (l *EarlyExitLoop) Call(ctx context.Context, work func() int) (int, error) {
	return call(ctx, l.requests, l.done, work)
}

// Cleaned reports whether cleanup ran.
func (l *EarlyExitLoop) Cleaned() bool { return l.cleaned.Load() }

// reply never blocks: Reply has room for one result and each request is answered once.
func reply(request Request) {
	select {
	case request.Reply <- request.Work():
	default:
	}
}

func call(
	ctx context.Context, requests chan<- Request, done <-chan struct{}, work func() int,
) (int, error) {
	request := Request{Work: work, Reply: make(chan int, 1)}
	select {
	case requests <- request:
	case <-done:
		return 0, ErrClosed
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	// The buffer can accept work the loop never reads, so done must end this wait.
	select {
	case result := <-request.Reply:
		return result, nil
	case <-done:
		// done can win the select against a reply sent just before it closed.
		select {
		case result := <-request.Reply:
			return result, nil
		default:
		}
		return 0, ErrClosed
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
