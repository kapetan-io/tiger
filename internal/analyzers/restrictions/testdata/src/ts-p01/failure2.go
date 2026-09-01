// A second //tiger:restrict directive in this file's own package doc
// comment: TS-P01 fires at the directive, naming the duplication.
//
// want +2 `TS-P01: package tsp01 has two //tiger:restrict directives`
//
//tiger:restrict closed-dispatch
package tsp01
