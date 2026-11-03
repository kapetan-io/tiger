package loopholes



func OrCounter(done func() bool) {
	for i := 0; i < 10 || !done(); i++ {
	}
}


func CounterReset() {
	for i := 0; i < 10; i++ {
		i = 0
	}
}

type register struct{ i int }



func FieldNamedLikeCounter(r register) {
	for i := 0; r.i < 10; i++ {
	}
}


func StepsPastLimit() {
	for i := 0; i != 7; i += 2 {
	}
}


func ByteWraps() {
	for i := uint8(0); i <= 255; i++ {
	}
}


func Register() register { return register{} }
