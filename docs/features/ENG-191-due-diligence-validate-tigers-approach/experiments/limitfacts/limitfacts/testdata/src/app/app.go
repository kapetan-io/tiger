package app

import (
	"errors"
	"lib"
	"math"
	"store"
	"types"
)

const spinsMax = 10_000

const listMax = 1_000

type SpinConfig struct{ Spins int }

type ListRequest struct{ Limit int32 }

var errTooLarge = errors.New("limit too large")

func SpinFromConfig(config SpinConfig, done func() bool) error {
	return lib.SpinReported(done, min(config.Spins, spinsMax))
}

func SpinFromConfigRaw(config SpinConfig, done func() bool) error {
	return lib.SpinReported(done, config.Spins) // want `has no upper bound where it enters`
}

func SpinForever(done func() bool) error {
	return lib.Spin(done, math.MaxInt) // want `too large for the loop ever to reach`
}

// Forward passes its parameter on, so the fact moves to its callers.
func Forward(done func() bool, n int) error { // want Forward:`\[1\]`
	return lib.SpinReported(done, n)
}

func ForwardCallers(config SpinConfig, done func() bool) {
	_ = Forward(done, 50)
	_ = Forward(done, config.Spins) // want `has no upper bound where it enters`
	_ = Forward(done, min(config.Spins, spinsMax))
}

func Computed(items []string, done func() bool) error {
	return lib.SpinReported(done, len(items)*2)
}

func ListClamped(lister store.Lister, rows store.Rows, req ListRequest) (int, error) {
	if req.Limit > listMax {
		return 0, errTooLarge
	}
	return lister.List(rows, types.ListOptions{Limit: int(req.Limit)}), nil
}

func ListRaw(lister store.Lister, rows store.Rows, req ListRequest) int {
	if req.Limit == 0 {
		req.Limit = listMax
	}
	return lister.List(rows, types.ListOptions{Limit: int(req.Limit)}) // want `has no upper bound where it enters`
}

func ListAssigned(rows store.Rows, req ListRequest) int {
	var opts types.ListOptions
	opts.Limit = int(req.Limit) // want `has no upper bound where it enters`
	return store.Memory{}.List(rows, opts)
}

// Known miss: a call through a function value carries no fact.
func ThroughValue(config SpinConfig, done func() bool) error {
	spin := lib.SpinReported
	return spin(done, config.Spins)
}

// Known miss: a clamp in another function does not dominate the sink, so this
// is flagged even though validate rejects large values. A false finding.
func validate(req ListRequest) error {
	if req.Limit > listMax {
		return errTooLarge
	}
	return nil
}

func ListValidatedElsewhere(lister store.Lister, rows store.Rows, req ListRequest) (int, error) {
	if err := validate(req); err != nil {
		return 0, err
	}
	return lister.List(rows, types.ListOptions{Limit: int(req.Limit)}), nil // want `has no upper bound where it enters`
}
