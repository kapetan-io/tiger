package fixture

// tagMax is used only by a file another build tag selects, which this pass
// never loads; the guard-only check counts that use.
const tagMax = 5

const _ = uint(writeBufferSize - entrySize*tagMax)
