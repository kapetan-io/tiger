package fixture

import "context"

// awaitParamOverwritten replaces its ctx parameter with Background.
//
// known-miss: a parameter counts as supplied by the caller wherever it is
// assigned, so the overwrite is not followed.
func awaitParamOverwritten(ctx context.Context, values chan int) {
	ctx = context.Background()
	select {
	case <-ctx.Done():
	case <-values:
	}
}

// Relay sends on out from two selects, and each send is the other's evidence.
type Relay struct {
	out chan int
}

// First sends once.
//
// known-miss: the send in Second counts as the package sending on out, and the
// send here counts the same for Second, though neither can stop the other.
func (r *Relay) First(value int) {
	select {
	case r.out <- value:
	}
}

// Second sends once.
func (r *Relay) Second(value int) {
	select {
	case r.out <- value:
	}
}
