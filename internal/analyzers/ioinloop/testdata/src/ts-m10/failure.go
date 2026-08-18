// The TS-M10 failure modes: stdlib IO called inside a for body, a range
// body, and an inner nested loop annotated only on its outer loop (the
// nested-loop exclusion — an inner loop needs its own directive).
package fixture

import (
	"database/sql"
	"net/http"
	"os"
)

// readAllFor calls os.ReadFile once per index inside a for body.
func readAllFor(paths []string) error {
	for i := 0; i < len(paths); i++ {
		_, err := os.ReadFile(paths[i]) // want `TS-M10: this call resolves to a package on the IO allowlist`
		if err != nil {
			return err
		}
	}
	return nil
}

// fetchAllRange calls the http client once per URL inside a range body.
func fetchAllRange(client *http.Client, urls []string) error {
	for _, url := range urls {
		resp, err := client.Get(url) // want `TS-M10: this call resolves to a package on the IO allowlist`
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	return nil
}

// queryAllRange calls a database/sql method once per id inside a range
// body.
func queryAllRange(db *sql.DB, ids []int) error {
	for _, id := range ids {
		rows, err := db.Query("SELECT 1 WHERE id = ?", id) // want `TS-M10: this call resolves to a package on the IO allowlist`
		if err != nil {
			return err
		}
		rows.Close() // want `TS-M10: this call resolves to a package on the IO allowlist`
	}
	return nil
}

// readNested annotates only the outer loop: the directive waives the outer
// loop's own body, not the inner loop's — the inner IO still fires.
func readNested(groups [][]string) error {
	//tiger:batched groups originate from an external drop directory; each read is required I/O
	for _, group := range groups {
		for _, path := range group {
			_, err := os.ReadFile(path) // want `TS-M10: this call resolves to a package on the IO allowlist`
			if err != nil {
				return err
			}
		}
	}
	return nil
}
