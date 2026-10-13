// Further shapes inside the leading t/ctx exemption (ADR-0018).
package fixture

import (
	"context"
	"testing"
)

type fixtureRunner interface {
	Run(t *testing.T, ctx context.Context, a int, s string, c float64, d byte)
}

type runner struct{}

func (runner) run(t testing.TB, ctx context.Context, a int, s string, c float64, d byte) {}

func testOnly(t *testing.T, a int, s string, c float64) {}

func fourPlusT(t *testing.T, a int, s string, c float64, d byte) {}

func ctxAndFour(ctx context.Context, a int, s string, c float64, d byte) {}
