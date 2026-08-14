// Package fixture holds a TS-S21 known miss: a tautological relational
// assertion that mentions its own limit but relates it to nothing else.
package fixture

// queueMax bounds how many pending items a queue may hold, but nothing
// downstream is checked against that belief.
// known-miss: the assertion below only casts queueMax to uint, which can
// never fail and proves nothing about a relation. TS-S21 collects every
// identifier that appears inside a `const _ = uint(...)` expression without
// judging whether the expression relates the limit to anything else, so
// this tautology counts as participation and the rule stays silent.
const queueMax = 100

const _ = uint(queueMax)
