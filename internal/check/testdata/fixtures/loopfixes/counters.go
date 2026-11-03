package loopfixes


func OrCounter(done func() bool) int {
	hits := 0
	for i := 0; i < 10; i++ {
		if done() {
			hits++
		}
	}
	return hits
}


func CounterReset() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}


func WrongWay(n int) int {
	steps := 0
	for i := 0; i < n; i++ {
		steps++
	}
	return steps
}


func LimitGrows(n int) {
	for i := 0; i < n; i++ {
		_ = i
	}
}


func StepsPastLimit() {
	for i := 0; i < 7; i += 2 {
		_ = i
	}
}


func ByteWraps() {
	for i := uint8(0); i < 255; i++ {
		_ = i
	}
}


func Overflow() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}


type register struct{ limit int }


func FieldNamedLikeCounter(r register) {
	for i := 0; i < r.limit; i++ {
		_ = i
	}
}
