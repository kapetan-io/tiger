package loopdotless

import "myapp/seq"


func Sum() int {
	total := 0
	for value := range seq.Naturals() {
		total += value
	}
	return total
}
