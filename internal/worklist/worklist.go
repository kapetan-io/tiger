// Package worklist holds the limit every tiger worklist shares. A loop that
// walks a graph by appending to the slice it walks ends only when the graph
// does, so each such loop also stops at NodesMax and reports when it gets
// there; a cycle then fails loudly instead of hanging.
package worklist

// NodesMax is the most nodes any one worklist in tiger visits.
const NodesMax = 1 << 20

// The relation below is a compile-time check that NodesMax stays under 2^31,
// the largest count a 32-bit int holds. It stops compiling when NodesMax
// outgrows what it is meant to stay below. The other limits in tiger's
// analyzers use the same form for the same reason.
const _ = uint(1<<31 - NodesMax)
