// Package annotations shows what tiger checks when a loop carries
// //tiger:variant or //tiger:batched, one honest loop and one lying loop each.
package annotations

// Rows is the part of a database cursor the loops use, shaped like pgx.Rows.
type Rows interface {
	Next() bool
	Value() string
}
