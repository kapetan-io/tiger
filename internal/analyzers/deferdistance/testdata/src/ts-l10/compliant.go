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

// closePerFrame defers inside a closure spawned by a loop: each closure's
// defer resolves when its own frame returns, once per call, so nothing
// queues across iterations.
func closePerFrame(names []string) {
	for range names {
		go func() {
			r, err := openResource()
			if err != nil {
				return
			}
			defer r.Close()
		}()
	}
}

// releaseCaptured defers a release of something the enclosing frame
// acquired: the closure has no local acquisition to sit next to.
func releaseCaptured() error {
	r, err := openResource()
	if err != nil {
		return err
	}
	go func() {
		defer r.Close()
	}()
	return nil
}

// unlockImmediate releases right after a method-call acquisition: mu.Lock
// is the acquisition defer mu.Unlock releases.
func unlockImmediate() {
	sharedGate.Lock()
	defer sharedGate.Unlock()
}

// releaseAfterSetup tolerates a method call on the same identifier between
// its declaration and the defer: the call is the acquisition being
// released.
func releaseAfterSetup() error {
	tx, err := beginTransaction()
	if err != nil {
		return err
	}
	tx.Begin()
	defer tx.Rollback()
	return nil
}
