// More neighbours of the leading t/ctx exemption that still fire.
package fixture

import (
	"context"
	"testing"
)

type fixtureRunnerBad interface {
	Run(t *testing.T, ctx context.Context, a int, s string, c float64, d byte, e rune) // want `TS-N07: this function takes 5 parameters`
}

type runnerBad struct{}

func (runnerBad) run(t testing.TB, ctx context.Context, a int, s string, c float64, d byte, e rune) { // want `TS-N07: this function takes 5 parameters`
}

// ctx-only with five other parameters.
func ctxFiveOthers(ctx context.Context, a int, s string, c float64, d byte, e rune) { // want `TS-N07: this function takes 5 parameters`
}

// Testing handle after ctx: ctx is first and exempt, t is then not leading.
func ctxThenT(ctx context.Context, t *testing.T, a int, s string, c float64, d byte) { // want `TS-N07: this function takes 5 parameters`
}

// A *testing.M or *testing.F is not an exempt handle.
func mainHandle(m *testing.M, a int, s string, c float64, d byte) { // want `TS-N07: this function takes 5 parameters`
}

func fuzzHandle(f *testing.F, a int, s string, c float64, d byte) { // want `TS-N07: this function takes 5 parameters`
}

// A testing.T by value, not pointer, is not an exempt handle.
func byValueT(t testing.T, a int, s string, c float64, d byte) { // want `TS-N07: this function takes 5 parameters`
}

// Two testing handles: only the first is exempt.
func tAndTB(t *testing.T, tb testing.TB, a int, s string, c float64, d byte) { // want `TS-N07: this function takes 5 parameters`
}

// Variadic and grouped parameters count each name.
func groupedAfterExempt(t *testing.T, ctx context.Context, a int, s string, c float64, d byte, rest ...byte) { // want `TS-N07: this function takes 5 parameters`
}
