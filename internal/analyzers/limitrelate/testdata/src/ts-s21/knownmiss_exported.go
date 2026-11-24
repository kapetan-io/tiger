package fixture

// ExportedMax is guarded and mentioned nowhere else in this package.
// known-miss: another package may use an exported limit and the analyzer
// sees one package at a time, so the guard-only check skips it. An
// exported limit invented only to satisfy the guard passes.
const ExportedMax = 64

const _ = uint(ExportedMax)
