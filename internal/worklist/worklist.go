// Package worklist holds the limit every tiger worklist shares. A loop that
// walks a graph by appending to the slice it walks ends only when the graph
// does, so each such loop also stops at NodesMax and reports when it gets
// there; a cycle then fails loudly instead of hanging.
package worklist

// NodesMax is the most nodes any one worklist in tiger visits.
const NodesMax = 1 << 20

// Each limit in tiger's analyzers is paired with a const _ = uint(CEILING -
// limit) line, a compile-time check that stops compiling when the limit passes
// its ceiling. The ceiling is 2^20 for a walk over one expression and 2^31 for
// NodesMax, the largest count a 32-bit int holds.
const _ = uint(1<<31 - NodesMax)
