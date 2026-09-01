// Package smoke contains known violations for the plugin smoke test. Per-
// package rules must surface through a golangci-lint run: TS-S09, TS-M10,
// TS-S01, TS-P01 (this package declares no-reflect and imports reflect),
// and TS-F02 — which can only fire if helper's effects fact crossed the
// import. The whole-program rules must not: Orphan is an invariant asserted
// nowhere and violated by no test (TS-A07, TS-A09), and Store has exactly
// one implementation (TS-X01); only `tiger check` reports those three, and a
// CLI test proves it does on this same fixture.
//
//tiger:restrict no-reflect
package smoke

import (
	"os"
	"reflect"

	"smoke.example/plugin/assert"
	"smoke.example/plugin/helper"
)

// ID names one invariant.
type ID string

const (
	// Asserted is defended in production, which is what makes ID an
	// invariant type at all.
	Asserted ID = "asserted"
	// Orphan is declared and never asserted.
	Orphan ID = "orphan"
)

// Store persists bytes; diskStore is its only implementation.
type Store interface {
	Put(data []byte) int
}

type diskStore struct {
	written int
}

// Put records the bytes.
func (d *diskStore) Put(data []byte) int {
	d.written += len(data)
	assert.Invariant(Asserted, d.written >= len(data))
	return d.written
}

// Open returns the only store there is.
func Open() *diskStore {
	return &diskStore{written: 0}
}

// Kind names a value's kind through reflection, contradicting no-reflect.
func Kind(value any) string {
	return reflect.TypeOf(value).Kind().String()
}

// Notify claims purity but reaches the network through the helper.
//
//tiger:effects none
func Notify() {
	helper.Widen()
}

// Spin loops with a goto so nogoto fires.
func Spin(limit int) int {
	count := 0
begin:
	count++
	if count < limit {
		goto begin
	}
	return count
}

// ReadAll calls stdlib IO once per path inside a bounded, unannotated loop
// so ioinloop fires.
func ReadAll(paths []string) error {
	for i := 0; i < len(paths); i++ {
		if _, err := os.ReadFile(paths[i]); err != nil {
			return err
		}
	}
	return nil
}

// Countdown recurses so norecursion — an SSA analyzer riding the plugin's
// native buildssa dependency resolution — fires through a golangci-lint
// run.
func Countdown(n int) int {
	if n <= 0 {
		return 0
	}
	return n + Countdown(n-1)
}
