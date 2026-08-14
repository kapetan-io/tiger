// Package fixture holds TS-S22 compliant cases: derivations that still
// hold, constants with no derivation comment at all, and prose that reads
// like a formula but does not parse as one.
package fixture

const pageSize = 4096
const pagesPerSegment = 8

// segmentSize = pageSize * pagesPerSegment.
const segmentSize = 32768

const segmentHeaderBytes = 12
const recordSize = 4

// recordMax = (segmentSize - segmentHeaderBytes) / recordSize.
const recordMax = 8189

// pageCount is a fixed configuration value; it carries no derivation
// formula, so TS-S22 has nothing to scan for and stays silent.
const pageCount = 16

// frameHeaderBytes = 4 checksum + 4 length + 4 sequence.
const frameHeaderBytes = 12

// The grouped-declaration form with a correct derivation: doubleSize's doc
// comment attaches to its own ValueSpec, and the arithmetic still holds.
const (
	doubleBase = 1

	// doubleSize = pageSize << 1.
	doubleSize = 8192
)
