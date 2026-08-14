// known-miss: function literals are exempt from TS-N08 for the same reason
// as TS-N07 — their signatures are dictated by the API that consumes them,
// so a finding on one is unfixable at the literal.
package fixture

// literalSave takes a plain bool but is a literal, so it stays silent.
var literalSave = func(immediate bool) {
	_ = immediate
}
