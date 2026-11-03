// Package store carries one escape directive under a budget row of 3, so
// the row has slack for tiger budget --write to lower.
package store


func NotifyAll(hooks []string) {
	//tiger:batched provider offers no bulk endpoint; contract caps us at 10 rps
	for range hooks {
		notifyOne()
	}
}

func notifyOne() {}



func Boom() {
	panic("boom")
}
