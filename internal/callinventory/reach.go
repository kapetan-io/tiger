package callinventory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

const maxWalk = 5_000_000

func importReach(roots []*packages.Package) map[string]bool {
	reaches := map[string]bool{}
	packages.Visit(roots, nil, func(pkg *packages.Package) {
		found := pkg.PkgPath == "syscall"
		for _, path := range slices.Sorted(maps.Keys(pkg.Imports)) {
			found = found || reaches[pkg.Imports[path].PkgPath]
		}
		reaches[pkg.PkgPath] = found
	})
	return reaches
}

type bodyReach struct {
	known map[*ssa.Function]bool
	table *StdReach
}

func newBodyReach(table *StdReach) *bodyReach {
	return &bodyReach{known: map[*ssa.Function]bool{}, table: table}
}

var errWalkTooLarge = fmt.Errorf("a reach walk visited more than %d functions", maxWalk)

func (b *bodyReach) reaches(start *ssa.Function) (bool, error) {
	if answer, ok := b.known[start]; ok {
		return answer, nil
	}
	seen := map[*ssa.Function]bool{start: true}
	queue := []*ssa.Function{start}
	i := 0
	for ; i < len(queue) && i < maxWalk; i++ {
		fn := queue[i]
		if isSyscall(fn) || b.known[fn] {
			b.known[start] = true
			return true, nil
		}
		if _, settled := b.known[fn]; settled {
			continue
		}
		if len(fn.Blocks) == 0 && b.table != nil && b.table.member(fn) {
			b.known[start] = true
			return true, nil
		}
		for _, next := range referenced(fn) {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	if i == maxWalk && i < len(queue) {
		return false, errWalkTooLarge
	}
	for _, fn := range queue {
		b.known[fn] = false
	}
	return false, nil
}

func isSyscall(fn *ssa.Function) bool {
	var path string
	if pkg := fn.Package(); pkg != nil {
		path = pkg.Pkg.Path()
	} else if obj := fn.Object(); obj != nil && obj.Pkg() != nil {
		path = obj.Pkg().Path()
	} else {
		return false
	}
	return path == "syscall" || strings.HasPrefix(path, "internal/syscall")
}

func referenced(fn *ssa.Function) []*ssa.Function {
	var list []*ssa.Function
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			for _, op := range instr.Operands(nil) {
				if op == nil || *op == nil {
					continue
				}
				if next, ok := (*op).(*ssa.Function); ok {
					list = append(list, next)
				}
			}
		}
	}
	return append(list, fn.AnonFuncs...)
}
