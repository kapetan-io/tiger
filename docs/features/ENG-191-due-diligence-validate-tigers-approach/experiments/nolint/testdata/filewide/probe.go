// Package probe declares a restriction its dependency does not, so TS-P02
// reports at the package line, where a "no //nolint" finding would go.
//
//nolint:tiger // above the package clause, so it covers the whole file
//tiger:restrict closed-dispatch
package probe

import "example.com/probe/dep"

// Name has a labeled break, which TS-S09 reports.
func Name(grid [][]int) string {
outer:
	for _, row := range grid {
		for range row {
			break outer
		}
	}
	return dep.Name()
}
