package callinventory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// maxWalk bounds the functions one reach question may visit, so a malformed
// program fails the load instead of looping.
const maxWalk = 5_000_000

// importReach reports, for every package, whether it is syscall or imports
// syscall, directly or transitively. Visit runs the post function after a
// package's imports, so their answers are already recorded.
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

// bodyReach decides whether an outside free function statically reaches
// package syscall through its own body, following static calls, referenced
// functions and closures, but not interface calls.
type bodyReach struct {
	// known holds a settled answer per function. A search that finds nothing
	// settles every function it visited as false, since it expanded all of
	// them. A search that finds syscall settles only its start as true.
	known map[*ssa.Function]bool
}

func newBodyReach() *bodyReach {
	return &bodyReach{known: map[*ssa.Function]bool{}}
}

// errWalkTooLarge reports a reach question that visited maxWalk functions
// with more still queued.
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
	pkg := fn.Package()
	if pkg == nil {
		return false
	}
	path := pkg.Pkg.Path()
	return path == "syscall" || strings.HasPrefix(path, "internal/syscall")
}

// referenced lists the functions fn refers to and the closures it defines.
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
