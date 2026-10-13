package loopsounds

import "errors"

const (
	spinsMax   = 10_000
	listMax    = 1_000
	objectsMax = 1 << 20
)

const _ = uint(1<<31 - spinsMax)
const _ = uint(1<<31 - listMax)
const _ = uint(1<<31 - objectsMax)

// SpinConfig holds settings read from a file at startup.
type SpinConfig struct{ Spins int }

// ListRequest is a list request as decoded off the wire.
type ListRequest struct{ Limit int32 }

// ListOptions carries a page size into a store.
type ListOptions struct{ Limit int }

var errTooLarge = errors.New("limit too large")

// SpinFromConfig clamps the config value to spinsMax where it enters.
func SpinFromConfig(config SpinConfig, done func() bool) error {
	return Spin(done, min(config.Spins, spinsMax))
}

// CountPage counts at most opts.Limit rows from a stream.
func CountPage(next func() bool, opts ListOptions) int {
	count := 0
	for count < opts.Limit && next() {
		count++
	}
	return count
}

// ListClamped rejects a request limit above listMax before it is stored.
func ListClamped(next func() bool, req ListRequest) (int, error) {
	if req.Limit > listMax {
		return 0, errTooLarge
	}
	return CountPage(next, ListOptions{Limit: int(req.Limit)}), nil
}

// CountObjects clamps the count a header claims before it loops.
func CountObjects(header uint32) int {
	count := int(header)
	if count > objectsMax {
		count = objectsMax
	}
	total := 0
	for i := 0; i < count; i++ {
		total += i
	}
	return total
}
