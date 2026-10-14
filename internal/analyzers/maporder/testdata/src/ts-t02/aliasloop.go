package fixture

import (
	"fmt"
	"sort"
)

// The closure is made after the sort, but the enclosing loop calls it before
// the next sort, so it prints the new map-ordered tail.
func leakThroughClosureNextIteration(m map[string]int) {
	var keys []string
	var show func()
	for i := 0; i < 2; i++ {
		for k := range m { // want `TS-T02`
			keys = append(keys, k)
		}
		if show != nil {
			show()
		}
		sort.Strings(keys)
		show = func() { fmt.Println(keys) }
	}
}

func leakThroughPointerNextIteration(m map[string]int) {
	var keys []string
	var p *[]string
	for i := 0; i < 2; i++ {
		for k := range m { // want `TS-T02`
			keys = append(keys, k)
		}
		if p != nil {
			fmt.Println(*p)
		}
		sort.Strings(keys)
		p = &keys
	}
}

// Declared inside the enclosing loop, the slice starts fresh each pass.
func freshEachPass(m map[string]int) {
	for i := 0; i < 2; i++ {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		show := func() { fmt.Println(keys) }
		show()
	}
}
