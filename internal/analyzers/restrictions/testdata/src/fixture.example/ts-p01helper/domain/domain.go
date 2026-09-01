// Package domain claims every axis TS-P01's corpus package also claims, so
// importing it never weakens tsp01's TS-P02 bound — the ts-p01 corpus is
// about TS-P01 alone.
//
//tiger:restrict closed-dispatch, no-reflect, imports(none)
package domain

// Marker is a plain exported value the corpus imports.
const Marker = "domain"
