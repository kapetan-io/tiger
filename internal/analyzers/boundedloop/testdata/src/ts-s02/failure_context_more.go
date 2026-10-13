// More contexts that no caller cancels, and a type that only looks like one.
package fixture

import "context"

// untilTodoEnds waits on a context nobody can cancel.
func untilTodoEnds(frame func()) {
	ctx := context.TODO()
	for ctx.Err() == nil { // want `TS-S02: tiger can't tell how many times this loop runs`
		frame()
	}
}

// watcher has an Err method and is not a context.
type watcher struct{}

func (watcher) Err() error { return nil }

// untilWatcherEnds waits on a parameter whose type is not context.Context.
func untilWatcherEnds(w watcher, frame func()) {
	for w.Err() == nil { // want `TS-S02: tiger can't tell how many times this loop runs`
		frame()
	}
}

// untilDerivedBackgroundEnds waits on a context derived from one nobody can
// cancel from outside.
func untilDerivedBackgroundEnds(frame func()) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for ctx.Err() == nil { // want `TS-S02: tiger can't tell how many times this loop runs`
		frame()
	}
}
