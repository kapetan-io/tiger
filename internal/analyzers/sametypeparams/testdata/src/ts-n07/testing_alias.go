// A type alias of a testing handle or context is still exempt.
package fixture

import (
	"context"
	"testing"
)

type (
	handle = testing.TB
	scope  = context.Context
)

func aliased(t handle, ctx scope, a int, s string, c float64, d byte) {
	_, _, _, _, _, _ = t, ctx, a, s, c, d
}
