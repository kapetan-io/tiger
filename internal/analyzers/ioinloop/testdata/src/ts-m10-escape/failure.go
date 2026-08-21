// The escape-never-silent proof: ioinloop consumes //tiger:batched to
// waive TS-M10, but consumption and reporting live in different analyzers
// with no shared state — the directives analyzer's TS-L09-escape standing
// advisory still fires on this same annotated loop. Kept out of the ts-m10
// dir so each analyzer's run matches only its own want comments.
package fixture

import "os"

func readAllBatched(paths []string) error {
	// want +1 `TS-L09: escape //tiger:batched — "paths originate from an external drop directory; each read is required I/O" \(unverified claim; standing review\)`
	//tiger:batched paths originate from an external drop directory; each read is required I/O
	for i := 0; i < len(paths); i++ {
		_, err := os.ReadFile(paths[i])
		if err != nil {
			return err
		}
	}
	return nil
}
