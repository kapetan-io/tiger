// Package fixture holds TS-S22 known misses: derivation comments that parse
// as valid Go expressions but cannot be evaluated because an identifier
// inside them does not resolve to a package-level constant.
package fixture

var defaultTimeoutSeconds = 30

// fetchTimeoutSeconds = defaultTimeoutSeconds.
// known-miss: defaultTimeoutSeconds is a var, not a const, so it never
// resolves through pass.Pkg.Scope().Lookup to a *types.Const. The comment
// parses and reads exactly like a derivation, but TS-S22 has no value to
// evaluate it against and stays silent.
const fetchTimeoutSeconds = 30

// retryBudget = unconfiguredCeiling.
// known-miss: unconfiguredCeiling does not exist anywhere in this package.
// The expression parses as a single identifier, but resolution fails, so
// TS-S22 stays silent rather than guessing at what the author meant.
const retryBudget = 5
