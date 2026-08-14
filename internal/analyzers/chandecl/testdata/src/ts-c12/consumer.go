// consumer.go uses the channel types declared in compliant.go. Using a
// channel type in another file is not itself a declaration, so nothing
// fires here — only compliant.go and failure_extra.go declare channel
// types.
package fixture

func consume(events Events) int {
	total := 0
	for range events {
		total++
	}
	return total
}

func newJob() Job {
	return Job{done: make(chan struct{})}
}
