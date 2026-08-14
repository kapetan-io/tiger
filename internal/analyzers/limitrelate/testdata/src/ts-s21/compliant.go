// Package fixture holds TS-S21 compliant cases: Max/Min constants that
// participate in a compile-time relational assertion, and constants the
// rule does not reach at all.
package fixture

const writeBufferSize = 32768
const headerSizeBytes = 12
const entrySize = 4

// entryMax bounds how many entries fit in one write buffer.
const entryMax = 8189

const _ = uint(entryMax*entrySize - (writeBufferSize - headerSizeBytes))

const sectorSize = 512

// sectorMin is the fewest sectors a single write may span.
const sectorMin = 1

const _ = uint(sectorMin*sectorSize - sectorSize)

// localExample shows that TS-S21 only inspects package-level constants; a
// local Max-suffixed constant inside a function body is out of its scope.
func localExample() int {
	const localMax = 10
	return localMax
}
