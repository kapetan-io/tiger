// Package patterns embeds the pattern collection so the tiger binary can
// print a pattern without the adopter having tiger's source. Each
// subdirectory is one pattern: a PATTERN.md card, the Go package that shows
// the right shape beside the broken one, and the tests that prove both.
package patterns

import "embed"

// Collection holds every pattern directory, rooted at this package.
//
//go:embed *
var Collection embed.FS
