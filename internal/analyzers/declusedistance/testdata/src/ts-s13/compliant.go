// The compliant TS-S13 shapes: a declaration at its point of use, a
// declaration exactly 10 lines before its use (the boundary — TS-S13 fires
// only past it), and a declaration whose sole reference is inside a closure
// that itself sits close to the declaration even though the identifier
// inside the closure appears many lines further down.
package fixture

func immediateUse() int {
	total := 0
	total = total + 1
	return total
}

func boundaryUse() int {
	total := 0
	println("1")
	println("2")
	println("3")
	println("4")
	println("5")
	println("6")
	println("7")
	println("8")
	println("9")
	total = total + 1
	return total
}

// closureNear declares total right before spawning a closure: the closure
// is the first reference to total and sits one line below the declaration,
// so distance is measured to the closure's own line — not to the deeply
// nested increment inside it, over ten lines further down.
func closureNear() {
	total := 0
	go func() {
		println("a")
		println("b")
		println("c")
		println("d")
		println("e")
		println("f")
		println("g")
		println("h")
		println("i")
		println("j")
		println("k")
		println("l")
		total++
	}()
}
