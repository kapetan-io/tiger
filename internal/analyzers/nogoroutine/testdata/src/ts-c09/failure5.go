// An errgroup.Group start per loop iteration is a goroutine per item as well,
// with or without SetLimit, and TryGo counts the same as Go.
package fixture

import "golang.org/x/sync/errgroup"

func notifyErrgroup(entries []string) error {
	var group errgroup.Group
	for _, entry := range entries {
		group.Go(func() error { notifyOne(entry); return nil }) // want `TS-C09: this loop starts a goroutine per item`
	}
	return group.Wait()
}

func notifyErrgroupTry(entries []string) error {
	var group errgroup.Group
	for _, entry := range entries {
		group.TryGo(func() error { notifyOne(entry); return nil }) // want `TS-C09: this loop starts a goroutine per item`
	}
	return group.Wait()
}

func notifyErrgroupLimited(entries []string) error {
	var group errgroup.Group
	group.SetLimit(8)
	for _, entry := range entries {
		group.Go(func() error { notifyOne(entry); return nil }) // want `TS-C09: this loop starts a goroutine per item`
	}
	return group.Wait()
}
