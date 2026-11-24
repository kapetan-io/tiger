// Package fixture holds TS-S21 compliant cases: Max/Min constants that
// participate in a compile-time relational assertion and are also used by
// the code, and constants the rule does not reach at all. fillBatch and
// spanSectors exist so entryMax and sectorMin appear outside their guards.
package fixture

const writeBufferSize = 32768
const headerSizeBytes = 12
const entrySize = 4

// entryMax bounds how many entries fit in one write buffer.
const entryMax = 8189

const _ = uint(entryMax*entrySize - (writeBufferSize - headerSizeBytes))

func fillBatch() int {
	count := 0
	for count < entryMax {
		count++
	}
	return count
}

const sectorSize = 512

// sectorMin is the fewest sectors a single write may span.
const sectorMin = 1

const _ = uint(sectorMin*sectorSize - sectorSize)

func spanSectors(count int) bool {
	return count >= sectorMin
}

// localExample shows that TS-S21 only inspects package-level constants; a
// local Max-suffixed constant inside a function body is out of its scope.
func localExample() int {
	const localMax = 10
	return localMax
}
