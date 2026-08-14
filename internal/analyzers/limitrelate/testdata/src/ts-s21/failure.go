// Package fixture holds TS-S21 failure-mode cases: Max/Min constants that
// participate in no compile-time relational assertion anywhere in the
// package.
package fixture

// batchMax bounds how many entries one batch may hold.
const batchMax = 8189 // want `TS-S21: a limit that relates to nothing`

// retryMin is the fewest retries a caller may configure.
const retryMin = 1 // want `TS-S21: a limit that relates to nothing`
