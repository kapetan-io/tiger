package loopholes

import "math"



func ShadowSlice(data []byte) {
	for len(data) > 0 {
		data := data
		data = data[1:]
		_ = data
	}
}



func AliasAppend(data []int) {
	alias := &data
	for len(data) > 0 {
		*alias = append(*alias, 1)
		data = data[1:]
	}
}



func ClosureGrow(data []int) {
	grow := func() { data = append(data, 1) }
	for len(data) > 0 {
		grow()
		data = data[1:]
	}
}



func LabeledContinue(data []int) {
outer:
	for len(data) > 0 {
		for j := 0; j < 1; j++ {
			continue outer
		}
		data = data[1:]
	}
}



func Overflow() {
	n := math.MaxInt
	i := 0
	for i <= n {
		i++
	}
}
