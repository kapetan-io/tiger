// Package flagged exists to prove analyzer flags ride through the CLI: the
// exported participle below fires by default and is silenced by
// -participle.allow=preparing.
package flagged

// Preparing is deliberately named with a trailing participle.
type Preparing struct{}
