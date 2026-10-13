// Limits bounded where they enter: a constant, a length, a clamp against a
// declared maximum, and a function that returns a clamped value.
package fixture

import (
	"errors"
	"lib"
	"store"
	"types"
	"wire"
)

const (
	spinsMax      = 10_000
	listMax       = 1_000
	objectsMax    = 1 << 20
	partitionsMax = 256
)

const _ = uint(1<<31 - spinsMax)
const _ = uint(1<<31 - listMax)
const _ = uint(1<<31 - objectsMax)
const _ = uint(1<<31 - partitionsMax)

var errTooLarge = errors.New("limit too large")

// SpinFromConfig clamps the config value to spinsMax where it enters.
func SpinFromConfig(config SpinConfig, done func() bool) error {
	return lib.SpinReported(done, min(config.Spins, spinsMax))
}

// Computed passes arithmetic over a length.
func Computed(items []string, done func() bool) error {
	return lib.SpinReported(done, len(items)*2)
}

// ListClamped rejects a limit above listMax before it is stored.
func ListClamped(lister store.Lister, rows store.Rows, req ListRequest) (int, error) {
	if req.Limit > listMax {
		return 0, errTooLarge
	}
	return lister.List(rows, types.ListOptions{Limit: int(req.Limit)}), nil
}

// IngestClamped rejects a header count above objectsMax.
func IngestClamped(raw []byte) int { // want IngestClamped:`\[\]`
	count := int(wire.Uint32(raw[8:12]))
	if count > objectsMax {
		return 0
	}
	return parseObjects(raw, count)
}

// CountClamped clamps the count it read before it loops.
func CountClamped(raw []byte) int { // want CountClamped:`\[\]`
	count := packCount(raw)
	if count > objectsMax {
		count = objectsMax
	}
	total := 0
	for i := 0; i < count; i++ {
		total += i
	}
	return total
}

// CountGuarded only loops while the count is within the maximum.
func CountGuarded(raw []byte) int { // want CountGuarded:`\[\]`
	count := packCount(raw)
	total := 0
	if count <= objectsMax {
		for i := 0; i < count; i++ {
			total += i
		}
	}
	return total
}

// clampPartitions is a clamp in a helper; its result carries the bound.
func clampPartitions(requested int) int { // want clampPartitions:`\[\]`
	return min(requested, partitionsMax)
}

// CreateClamped clamps through the helper before looping.
func CreateClamped(requested int, done func() bool) error {
	return lib.SpinReported(done, clampPartitions(requested))
}

// ForwardClamped passes a clamped value to a limit parameter.
func ForwardClamped(done func() bool, n int) error {
	return Forward(done, min(n, spinsMax))
}

// halve is bounded whenever its argument is.
func halve(n int) int { // want halve:`\[0\]`
	return n / 2
}

// HalfOfLength passes a result that depends on a bounded argument.
func HalfOfLength(items []int, done func() bool) error {
	return lib.SpinReported(done, halve(len(items)))
}
