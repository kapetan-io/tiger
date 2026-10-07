// Package cursorbound compares a cursor loop bounded only by the database
// with one that restates the query's limit in its own header.
package cursorbound

// Rows is the part of a database cursor the list loops use, shaped like
// pgx.Rows and sql.Rows.
type Rows interface {
	Next() bool
	Value() string
}
