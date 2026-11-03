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


type SpinConfig struct{ Spins int }


type ListRequest struct{ Limit int32 }


type ListOptions struct{ Limit int }

var errTooLarge = errors.New("limit too large")


func SpinFromConfig(config SpinConfig, done func() bool) error {
	return Spin(done, min(config.Spins, spinsMax))
}


func CountPage(next func() bool, opts ListOptions) int {
	count := 0
	for count < opts.Limit && next() {
		count++
	}
	return count
}


func ListClamped(next func() bool, req ListRequest) (int, error) {
	if req.Limit > listMax {
		return 0, errTooLarge
	}
	return CountPage(next, ListOptions{Limit: int(req.Limit)}), nil
}


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
