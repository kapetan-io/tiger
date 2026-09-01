// Package other is the disallowed dependency in the ts-p01 corpus: outside
// the declared imports(...) subtree, so importing it fires TS-P01. It
// claims every axis itself, so it does not also weaken tsp01's TS-P02
// bound — the ts-p01 corpus is about TS-P01 alone.
//
//tiger:restrict closed-dispatch, no-reflect, imports(none)
package other

// Marker is a plain exported value the corpus imports.
const Marker = "other"
