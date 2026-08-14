// The TS-C09 failure mode: a goroutine spawned per loop iteration reacts
// directly to each item instead of running at its own pace.
package fixture

func notify(entries []string) {
	for _, entry := range entries {
		go notifyOne(entry) // want `TS-C09: goroutine spawned per loop iteration`
	}
}

func notifyOne(entry string) {}
