// The compliant rewrites: a defer immediately after its acquisition, a
// defer immediately after the standard err check, and a defer outside any
// loop entirely.
package fixture

func closeImmediate() error {
	r, err := openResource()
	if err != nil {
		return err
	}
	defer r.Close()
	return nil
}

func rollbackImmediate() error {
	tx, err := beginTransaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return nil
}

func closeOutsideLoop(names []string) error {
	for range names {
		// no per-iteration resource acquired here
	}
	r, err := openResource()
	if err != nil {
		return err
	}
	defer r.Close()
	return nil
}
