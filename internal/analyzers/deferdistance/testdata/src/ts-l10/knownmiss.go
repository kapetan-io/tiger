// known-miss: TS-L10-distance only tracks defers whose call is a selector on
// a plain identifier (f.Close, tx.Rollback). A deferred closure or a
// deferred package-level call never resolves to an identifier's acquisition
// statement, so a badly placed defer of either shape stays silent even
// though the same leak-visibility problem applies.
package fixture

import "os"

func closeViaClosure(name string) error {
	r, err := openResource()
	if err != nil {
		return err
	}
	println("opened", name)
	defer func() {
		r.Close()
	}()
	return nil
}

func removeViaPackageCall(name string) error {
	println("about to remove", name)
	defer os.Remove(name)
	return nil
}
