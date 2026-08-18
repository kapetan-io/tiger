// known-miss: three documented gaps in the package-identity classifier.
package fixture

import (
	"os"

	"rpcclient"
)

// callAllRPC calls an unlisted package's client once per id. rpcclient is
// not on the seed or per-repo allowlist, so its IO stays silent; a project
// using it would extend -ioinloop.packages=rpcclient.
func callAllRPC(ids []string) error {
	for _, id := range ids {
		if err := rpcclient.Call(id); err != nil {
			return err
		}
	}
	return nil
}

// readHelper is a same-package helper that performs the actual IO.
func readHelper(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// readAllViaHelper calls readHelper once per path. Single-package analysis
// builds no call graph, so IO hidden behind a same-package helper is
// invisible from the call site.
func readAllViaHelper(paths []string) error {
	for _, path := range paths {
		if _, err := readHelper(path); err != nil {
			return err
		}
	}
	return nil
}

// readAllAsync launches a closure per path. A FuncLit is a frame boundary
// (consistent with deferdistance): the closure's own IO is its own
// accounting, not the loop's.
func readAllAsync(paths []string) {
	for _, path := range paths {
		go func() {
			_, _ = os.ReadFile(path)
		}()
	}
}
