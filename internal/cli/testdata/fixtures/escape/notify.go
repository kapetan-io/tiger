// Package escape carries wave 1's one escape hatch, which surfaces as an
// advisory finding on every run.
package escape

// NotifyAll performs per-item IO the outside world forces.
func NotifyAll(hooks []string) {
	//tiger:batched provider offers no bulk endpoint; contract caps us at 10 rps
	for range hooks {
		notifyAllOne()
	}
}

func notifyAllOne() {}
