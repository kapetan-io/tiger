// The TS-C02 failure modes: goroutines with no owner and no supervisor.
package fixture

func launch() {
	go work() // want `TS-C02: bare goroutine`
}

// launchClosure spawns from inside a closure; the owner is still
// launchClosure, the FuncDecl the closure lexically sits in.
func launchClosure() {
	fn := func() {
		go work() // want `TS-C02: bare goroutine`
	}
	fn()
}

func work() {}
