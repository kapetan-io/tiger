// Package intrange reports the smallest and largest value an integer type
// holds, which the loop rules use to see that a counter reaches its limit
// before the counter wraps around.
package intrange

import (
	"go/types"
	"math/big"
)

// Span is an inclusive range of integers.
type Span struct {
	Low  *big.Int
	High *big.Int
}

// Of returns the range of values t's integer type holds. It reports false
// when t is not an integer type.
func Of(t types.Type, sizes types.Sizes) (Span, bool) {
	basic, isBasic := t.Underlying().(*types.Basic)
	if !isBasic || basic.Info()&types.IsInteger == 0 {
		return Span{}, false
	}
	bits := uint(sizes.Sizeof(basic) * 8)
	one := big.NewInt(1)
	if basic.Info()&types.IsUnsigned != 0 {
		return Span{
			Low:  big.NewInt(0),
			High: new(big.Int).Sub(new(big.Int).Lsh(one, bits), one),
		}, true
	}
	half := new(big.Int).Lsh(one, bits-1)
	return Span{Low: new(big.Int).Neg(half), High: new(big.Int).Sub(half, one)}, true
}
