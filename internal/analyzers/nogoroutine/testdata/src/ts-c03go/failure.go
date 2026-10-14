// A bare go statement is a goroutine start for TS-C03 too, so the package
// gets both findings on the same line.
package fixture

func work() {}

func launch() {
	go work() // want `TS-C02: this go statement starts a goroutine nobody owns` `TS-C03: this package starts goroutines but its tests have no goleak check`
}
