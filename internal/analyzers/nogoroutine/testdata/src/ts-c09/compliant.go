// The compliant rewrite: the loop appends to a bounded work slice instead
// of spawning, and a separate supervisor drains it at its own pace.
package fixture

const queueCapacity = 64

func notifyQueued(entries []string) {
	queue := make([]string, 0, queueCapacity)
	for _, entry := range entries {
		queue = append(queue, entry)
	}
	Supervise(queue)
}

// Supervise drains the queue at its own pace; it holds no GoStmt.
func Supervise(queue []string) {
	for _, entry := range queue {
		notifyQueuedOne(entry)
	}
}

func notifyQueuedOne(entry string) {}
