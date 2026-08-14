// The TS-L09 blocking failure modes: directives that do not parse. The
// vocabulary is closed, so a made-up verb is an error, not a comment.
// Expectations sit one line above each directive (+1) so the directive
// line itself stays exactly what a developer would write.
package fixture

// withdrawnBoundedEscape reaches for the escape the admission test
// withdrew; every terminating loop can state a cap instead.
func withdrawnBoundedEscape(estimates []float64) float64 {
	refined := 0.0
	// want +1 `TS-L09: unknown directive verb //tiger:bounded`
	//tiger:bounded trust me, it converges
	for _, estimate := range estimates {
		refined += estimate
	}
	return refined
}

// ruleIDDismissalAttempt tries the generic deviation form, which does not
// ship before the heuristic waves and their ratchet.
func ruleIDDismissalAttempt(entries []int) int {
	total := 0
	// want +1 `TS-L09: unknown directive verb //tiger:TS-N07`
	//tiger:TS-N07 trust me
	for _, entry := range entries {
		total += entry
	}
	return total
}

// escapeWithoutReason declares the world constraint but not the reason,
// which is the point of the declaration.
func escapeWithoutReason(hooks []string) {
	// want +1 `TS-L09: //tiger:batched carries no reason`
	//tiger:batched
	for range hooks {
		notifyOne()
	}
}

// withdrawnSkipVerb reaches for the skip declaration TS-D07 no longer uses:
// every skipped test now surfaces as a standing advisory instead, so the
// verb left the vocabulary.
//
// want +1 `TS-L09: unknown directive verb //tiger:skip`
//tiger:skip ENG-147 needs the fake clock
func withdrawnSkipVerb() {}

func notifyOne() {}
