// Package fixture holds TS-S22 failure-mode cases: derivation comments
// whose arithmetic no longer matches the constant it documents.
package fixture

const sectorSize = 512
const sectorsPerWrite = 64
const headerSizeBytes = 12
const entrySize = 4

// writeBufferSize = sectorSize * sectorsPerWrite.
const writeBufferSize = 32768

// batchMax = (writeBufferSize - headerSizeBytes) / entrySize.
const batchMax = 8188 // want `TS-S22: batchMax is 8188 but its stated derivation \(writeBufferSize - headerSizeBytes\) / entrySize evaluates to 8189`

// The grouped-declaration form: blockMax's doc comment attaches directly to
// its own ValueSpec rather than falling back to the GenDecl's Doc, since it
// is not the first spec in the block.
const (
	blockBase = 1

	// blockMax = sectorSize << 1.
	blockMax = 999 // want `TS-S22: blockMax is 999 but its stated derivation sectorSize << 1 evaluates to 1024`
)
