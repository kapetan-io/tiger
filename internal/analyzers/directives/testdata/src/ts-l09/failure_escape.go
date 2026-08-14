// The TS-L09 advisory case: a well-formed escape surfaces on every run —
// escapes are never silent. The reason is shape-checked here and
// truth-reviewed by humans, so the claim stays on the report.
package fixture

// notifyAll performs per-item IO the outside world forces; the escape
// stays visible as a standing advisory finding.
func notifyAll(hooks []string) {
	// want +1 `TS-L09: escape //tiger:batched — "provider offers no bulk endpoint; contract caps us at 10 rps" \(unverified claim; standing review\)`
	//tiger:batched provider offers no bulk endpoint; contract caps us at 10 rps
	for range hooks {
		notifyOne()
	}
}
