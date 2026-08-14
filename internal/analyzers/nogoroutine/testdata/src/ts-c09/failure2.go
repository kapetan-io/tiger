// A counter-style for loop reacting to each iteration is the same
// reactive anti-pattern as a range loop.
package fixture

func poll(count int) {
	for i := 0; i < count; i++ {
		go notifyOne("tick") // want `TS-C09: goroutine spawned per loop iteration`
	}
}
