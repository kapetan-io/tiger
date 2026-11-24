package cgocycle

import "C"

func Apple(n int) int {
	return Zebra(n) + int(C.int(0))
}
