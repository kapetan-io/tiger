// Package ts02helper claims no-reflect only, leaving closed-dispatch and
// imports(...) unclaimed — the ts-p02 corpus's partially weakening
// dependency.
//
//tiger:restrict no-reflect
package ts02helper

// Marker is a plain exported value the corpus imports.
const Marker = "helper"
