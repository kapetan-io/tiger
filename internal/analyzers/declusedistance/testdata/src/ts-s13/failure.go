// The TS-S13 failure modes: a declaration separated from its first use by
// more than 10 lines, for a short declaration, a var declaration, and a
// bare method call as the qualifying use.
package fixture

func farAssignUse() int {
	total := 0 // want `TS-S13: total is declared 11 lines before its first use`
	println("1")
	println("2")
	println("3")
	println("4")
	println("5")
	println("6")
	println("7")
	println("8")
	println("9")
	println("10")
	total = total + 1
	return total
}

func farVarUse() int {
	var total int // want `TS-S13: total is declared 11 lines before its first use`
	println("1")
	println("2")
	println("3")
	println("4")
	println("5")
	println("6")
	println("7")
	println("8")
	println("9")
	println("10")
	total = 5
	return total
}

func farMethodUse() {
	c := newCounter() // want `TS-S13: c is declared 11 lines before its first use`
	println("1")
	println("2")
	println("3")
	println("4")
	println("5")
	println("6")
	println("7")
	println("8")
	println("9")
	println("10")
	c.Inc()
}
