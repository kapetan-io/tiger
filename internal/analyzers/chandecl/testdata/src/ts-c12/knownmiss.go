// known-miss: anonymous channel types that appear only in a function
// signature or a var declaration are not counted as declarations. TS-C12
// governs named type declarations (TypeSpec), not every place a channel
// type is spelled out, so a package whose only chan usage is ad hoc like
// this never fires even though both symbols below write out their own
// chan type.
package fixture

func worker(jobs chan int) int {
	total := 0
	for job := range jobs {
		total += job
	}
	return total
}

var pings = make(chan struct{})
