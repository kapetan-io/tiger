// known-miss: a FuncLit is a frame boundary, so a variable's distance is
// measured to the position of the first closure that references it, never
// to the line inside that closure where the reference actually sits. When
// a handful of unrelated statements separate the declaration from the
// closure — fewer than the threshold — the closure "rescues" a variable
// that, read as ordinary control flow, is untouched for most of the
// function body: total sits idle through five setup lines and then through
// ten more lines inside the closure before its only reference, yet TS-S13
// stays silent because the closure itself is only six lines from the
// declaration.
package fixture

func rescuedByClosure() {
	total := 0
	println("setup 1")
	println("setup 2")
	println("setup 3")
	println("setup 4")
	println("setup 5")
	go func() {
		println("work 1")
		println("work 2")
		println("work 3")
		println("work 4")
		println("work 5")
		println("work 6")
		println("work 7")
		println("work 8")
		println("work 9")
		println("work 10")
		total++
	}()
}
