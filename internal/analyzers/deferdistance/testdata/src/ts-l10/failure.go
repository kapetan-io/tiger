// The TS-L10 failure modes: a defer inside a loop, and a defer whose
// acquisition sits behind an unrelated statement or in a different block.
package fixture

func closeAllFor(names []string) error {
	for i := 0; i < len(names); i++ {
		r, err := openResource()
		if err != nil {
			return err
		}
		defer r.Close() // want `TS-L10: defer inside a loop`
	}
	return nil
}

func closeAllRange(names []string) error {
	for range names {
		r, err := openResource()
		if err != nil {
			return err
		}
		defer r.Close() // want `TS-L10: defer inside a loop`
	}
	return nil
}

// closeAllInWorker loops inside a closure: the frame reset starts a fresh
// depth, and the loop inside it still queues cleanup without bound.
func closeAllInWorker(names []string) {
	go func() {
		for range names {
			r, err := openResource()
			if err != nil {
				return
			}
			defer r.Close() // want `TS-L10: defer inside a loop`
		}
	}()
}

func closeAfterUnrelated(name string) error {
	r, err := openResource()
	if err != nil {
		return err
	}
	println("opened", name)
	defer r.Close() // want `TS-L10: defer sits away from its acquisition`
	return nil
}

func closeConditional(name string, verbose bool) error {
	r, err := openResource()
	if err != nil {
		return err
	}
	if verbose {
		defer r.Close() // want `TS-L10: defer sits away from its acquisition`
	}
	_ = name
	return nil
}
