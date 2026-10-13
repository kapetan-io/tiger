// A leading testing handle, and a context first or right after it, do not
// count toward the four-parameter limit (ADR-0018).
package fixture

import (
	"context"
	"testing"
)

func helperTCtx(t *testing.T, ctx context.Context, a int, b string, c float64, d byte) {
	_, _, _, _, _, _ = t, ctx, a, b, c, d
}

func helperBCtx(b *testing.B, ctx context.Context, a int, s string, c float64, d byte) {
	_, _, _, _, _, _ = b, ctx, a, s, c, d
}

func helperTB(tb testing.TB, a int, s string, c float64, d byte) {
	_, _, _, _, _ = tb, a, s, c, d
}

func helperCtxOnly(ctx context.Context, a int, s string, c float64, d byte) {
	_, _, _, _, _ = ctx, a, s, c, d
}
