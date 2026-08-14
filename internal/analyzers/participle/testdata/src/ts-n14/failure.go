// The TS-N14 failure modes: exported identifiers whose last words.Split
// token ends in a present participle that is not a genuine noun.
package fixture

// Preparing is a single-token exported type name ending in a participle.
type Preparing struct{} // want `TS-N14: "Preparing" ends in the participle "Preparing"`

// StartProcessing is an exported function whose last token is a
// participle.
func StartProcessing() { // want `TS-N14: "StartProcessing" ends in the participle "Processing"`
}

// Retrying is an exported const built entirely from a participle.
const Retrying = true // want `TS-N14: "Retrying" ends in the participle "Retrying"`

// Loading is an exported package-level var ending in a participle.
var Loading bool // want `TS-N14: "Loading" ends in the participle "Loading"`

// Job carries an exported field whose last token is a participle.
type Job struct {
	// Pending is the exported field.
	Pending bool // want `TS-N14: "Pending" ends in the participle "Pending"`
}

// Reader has an exported method whose name ends in a participle.
type Reader struct{}

// Closing is an exported method name built from a participle.
func (Reader) Closing() { // want `TS-N14: "Closing" ends in the participle "Closing"`
}
