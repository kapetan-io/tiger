// Stop cases that leave the loop other than by a trailing return.
package fixture

import "context"

func runBreaksOut(ctx context.Context, work <-chan int) int {
	total := 0
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case n := <-work:
			total += n
		}
	}
	return total
}

func runCleansUpThenReturns(ctx context.Context, work <-chan int) error {
	for {
		select {
		case <-ctx.Done():
			drain(work)
			return ctx.Err()
		case <-work:
		}
	}
}

func drain(work <-chan int) {
	for range len(work) {
		<-work
	}
}
