package shutdownloop

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var ErrClosed = errors.New("shutdownloop: loop closed")

type Request struct {
	Work  func() int
	Reply chan int
}

type Loop struct {
	requests chan Request
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	cleaned  atomic.Bool
	wg       sync.WaitGroup
}

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

func (l *Loop) Call(ctx context.Context, work func() int) (int, error) {
	return call(ctx, l.requests, l.done, work)
}

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

func (l *Loop) Cleaned() bool { return l.cleaned.Load() }

type EarlyExitLoop struct {
	requests   chan Request
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
	inShutdown atomic.Bool
	cleaned    atomic.Bool
	wg         sync.WaitGroup
}

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

func (l *EarlyExitLoop) Call(ctx context.Context, work func() int) (int, error) {
	return call(ctx, l.requests, l.done, work)
}

func (l *EarlyExitLoop) Cleaned() bool { return l.cleaned.Load() }

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
	select {
	case result := <-request.Reply:
		return result, nil
	case <-done:
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
