// Package intrange reports the smallest and largest value an integer type
// holds, which the loop rules use to see that a counter reaches its limit
// before the counter wraps around.
package intrange

import (
	"go/ast"
	"go/constant"
	"go/types"
	"math/big"

	"github.com/kapetan-io/tiger/assert"
)

// Span is an inclusive range of integers.
type Span struct {
	Low  *big.Int
	High *big.Int
}

// termsMax bounds how many constraint terms a type parameter is unpacked
// into.
const termsMax = 1 << 10

const _ = uint(1<<20 - termsMax)

// IsInteger reports whether t is an integer type, or a type parameter whose
// constraint allows only integer types.
func IsInteger(t types.Type) bool {
	basics, ok := basicsOf(t)
	if !ok {
		return false
	}
	for _, basic := range basics {
		if basic.Info()&types.IsInteger == 0 {
			return false
		}
	}
	return true
}

// Of returns the range of values t's integer type holds. For a type
// parameter it is the range every allowed type holds. It reports false when
// t is not an integer type. An untyped constant counts as an int.
func Of(t types.Type, sizes types.Sizes) (Span, bool) {
	if !IsInteger(t) {
		return Span{}, false
	}
	basics, _ := basicsOf(t)
	var common Span
	for _, basic := range basics {
		span := spanOf(basic, sizes)
		if common.Low == nil || span.Low.Cmp(common.Low) > 0 {
			common.Low = span.Low
		}
		if common.High == nil || span.High.Cmp(common.High) < 0 {
			common.High = span.High
		}
	}
	return common, true
}

// OutOfReach reports whether the span reaches at least as high as an int32
// does, which no loop counts up to in practice. A narrower type can be
// counted to its edge, and a constant in the upper half of a wider one cannot.
func (s Span) OutOfReach() bool {
	return s.High.Cmp(big.NewInt(1<<31-1)) >= 0
}

// spanOf returns the range of one integer type.
func spanOf(basic *types.Basic, sizes types.Sizes) Span {
	if basic.Info()&types.IsUntyped != 0 {
		basic = types.Typ[types.Int]
	}
	bits := uint(sizes.Sizeof(basic) * 8)
	one := big.NewInt(1)
	if basic.Info()&types.IsUnsigned != 0 {
		return Span{
			Low:  big.NewInt(0),
			High: new(big.Int).Sub(new(big.Int).Lsh(one, bits), one),
		}
	}
	half := new(big.Int).Lsh(one, bits-1)
	return Span{Low: new(big.Int).Neg(half), High: new(big.Int).Sub(half, one)}
}

// basicsOf returns the basic types t can be: its own underlying type, or each
// type its constraint allows when it is a type parameter. It reports false
// when t can be anything that is not basic.
func basicsOf(t types.Type) ([]*types.Basic, bool) {
	param, isParam := t.(*types.TypeParam)
	if !isParam {
		basic, ok := t.Underlying().(*types.Basic)
		return []*types.Basic{basic}, ok
	}
	var basics []*types.Basic
	pending := []types.Type{param.Constraint()}
	steps := 0
	for ; len(pending) > 0 && steps < termsMax; steps++ {
		last := len(pending) - 1
		next := pending[last].Underlying()
		pending = pending[:last]
		switch found := next.(type) {
		case *types.Interface:
			for i := 0; i < found.NumEmbeddeds(); i++ {
				pending = append(pending, found.EmbeddedType(i))
			}
		case *types.Union:
			for i := 0; i < found.Len(); i++ {
				pending = append(pending, found.Term(i).Type())
			}
		case *types.Basic:
			basics = append(basics, found)
		default:
			return nil, false
		}
	}
	assert.Ok(len(pending) == 0, "basicsOf: constraint has more than termsMax terms")
	return basics, len(basics) > 0
}

// SmallConstant returns expr's value when the compiler folded it to an
// integer that fits an int64.
func SmallConstant(info *types.Info, expr ast.Expr) (int64, bool) {
	value := info.Types[expr].Value
	if value == nil || value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(value)
}
