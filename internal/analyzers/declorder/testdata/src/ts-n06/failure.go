// The TS-N06 failure modes: a helper with exactly one caller whose name
// does not carry that caller's prefix.
package fixture

// ReadSector calls retryHelper, its single caller in the package.
func ReadSector(id int) []byte {
	return retryHelper(id)
}

func retryHelper(id int) []byte { // want `TS-N06: retryHelper has a single caller \(ReadSector\)`
	return nil
}

// runQuery calls buildFilter, its single caller in the package.
func runQuery(term string) string {
	return buildFilter(term)
}

func buildFilter(term string) string { // want `TS-N06: buildFilter has a single caller \(runQuery\)`
	return "filter:" + term
}
