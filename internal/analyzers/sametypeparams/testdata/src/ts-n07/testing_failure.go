// Neighbours of the leading t/ctx exemption that still fire.
package fixture

import (
	"context"
	"testing"
)

// A t after the first position counts.
func tLate(a int, t *testing.T, s string, c float64, d byte) { // want `TS-N07: this function takes 5 parameters`
	_, _, _, _, _ = a, t, s, c, d
}

// A ctx that is neither first nor right after the testing handle counts.
func ctxLate(a int, ctx context.Context, s string, c float64, d byte) { // want `TS-N07: this function takes 5 parameters`
	_, _, _, _, _ = a, ctx, s, c, d
}

// Only one testing handle and one context are exempt.
func twoContexts(ctx context.Context, other context.Context, a int, s string, c float64, d byte) { // want `TS-N07: adjacent parameters share type context.Context` `TS-N07: this function takes 5 parameters`
}

// Exempting t and ctx still leaves five counted parameters.
func stillTooMany(t *testing.T, ctx context.Context, a int, s string, c float64, d byte, e rune) { // want `TS-N07: this function takes 5 parameters`
}

// Adjacent same-type parameters fire even right after the exempt pair.
func adjacentAfter(t *testing.T, ctx context.Context, a, b int) { // want `TS-N07: adjacent parameters share type int`
}

// Adjacent testing handles still share a type.
func twoTs(t *testing.T, u *testing.T) { // want `TS-N07: adjacent parameters share type \*testing.T`
}
