package twofiles

// Zebra and Apple form one cycle whose members sit in two files. The finding
// belongs at Zebra, the member in the file that sorts first by name, though
// Apple sorts first alphabetically.
func Zebra(n int) int { // want `TS-S01: Apple → Zebra → Apple is a cycle of calls`
	return Apple(n)
}
