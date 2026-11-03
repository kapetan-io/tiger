package loopholes

import "iter"



func naturals() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < len(naturalsForever); i++ {
			if !yield(i) {
				return
			}
			i--
		}
	}
}


var naturalsForever = []int{0}


func RangeNaturals() {
	for value := range naturals() {
		_ = value
	}
}


func GotoLoop() {
loop:
	goto loop
}
