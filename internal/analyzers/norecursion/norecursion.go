// Package norecursion enforces TS-S01: no cycle in the package's static call
// graph.
package norecursion

import (
	"cmp"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/ssalib"
	"github.com/kapetan-io/tiger/internal/worklist"
)

const msgRewrite = " is a cycle of calls (recursion) — replace it with a capped worklist loop " +
	"that returns an error when it stops at max with nodes left"

var Analyzer = &analysis.Analyzer{
	Name:     "norecursion",
	Doc:      "TS-S01: no cycle in the package's static call graph.",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	built, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	assert.Ok(ok, "buildssa result missing")
	nodes := collectFunctions(built.SrcFuncs)

	callees := map[*ssa.Function][]*ssa.Function{}
	for _, fn := range nodes {
		called := []*ssa.Function{}
		for _, call := range ssalib.StaticCalls(fn) {
			called = append(called, sourceFunc(forwardedTarget(call.Callee)))
		}
		callees[fn] = called
	}

	components := ssalib.Components(nodes, func(fn *ssa.Function) []*ssa.Function {
		return callees[fn]
	})
	for _, component := range components {
		if isCycle(component, callees) {
			report(pass, component)
		}
	}
	return nil, nil
}

func forwardedTarget(fn *ssa.Function) *ssa.Function {
	if !strings.HasPrefix(fn.Synthetic, "bound method wrapper") &&
		!strings.HasPrefix(fn.Synthetic, "thunk for") {
		return fn
	}
	forwarded := ssalib.StaticCalls(fn)
	if len(forwarded) != 1 {
		return fn
	}
	return forwarded[0].Callee
}

func sourceFunc(fn *ssa.Function) *ssa.Function {
	if origin := fn.Origin(); origin != nil {
		return origin
	}
	return fn
}

func collectFunctions(sourceFuncs []*ssa.Function) []*ssa.Function {
	seen := map[*ssa.Function]bool{}
	collected := []*ssa.Function{}
	for _, fn := range sourceFuncs {
		if !seen[fn] {
			seen[fn] = true
			collected = append(collected, fn)
		}
	}
	i := 0
	for ; i < len(collected) && i < worklist.NodesMax; i++ {
		for _, anon := range collected[i].AnonFuncs {
			if !seen[anon] {
				seen[anon] = true
				collected = append(collected, anon)
			}
		}
	}
	assert.Ok(i == len(collected), "collectFunctions: worklist reached worklist.NodesMax")
	return collected
}

func isCycle(component []*ssa.Function, callees map[*ssa.Function][]*ssa.Function) bool {
	if len(component) > 1 {
		return true
	}
	self := component[0]
	return slices.Contains(callees[self], self)
}

func report(pass *analysis.Pass, component []*ssa.Function) {
	sorted := sortedByName(pass.Fset, component)
	names := make([]string, 0, len(sorted)+1)
	for _, fn := range sorted {
		names = append(names, fn.Name())
	}
	names = append(names, sorted[0].Name())

	pass.Report(analysis.Diagnostic{
		Pos:      firstPos(pass.Fset, component),
		Category: "TS-S01",
		Message:  "TS-S01: " + strings.Join(names, " → ") + msgRewrite,
	})
}

func sortedByName(fset *token.FileSet, component []*ssa.Function) []*ssa.Function {
	sorted := slices.Clone(component)
	slices.SortFunc(sorted, func(a, b *ssa.Function) int {
		if a.Name() != b.Name() {
			return strings.Compare(a.Name(), b.Name())
		}
		left := place(fset.PositionFor(a.Pos(), true))
		return left.compare(place(fset.PositionFor(b.Pos(), true)))
	})
	return sorted
}

func firstPos(fset *token.FileSet, component []*ssa.Function) token.Pos {
	smallest := component[0].Pos()
	for _, fn := range component[1:] {
		here := place(fset.PositionFor(fn.Pos(), true))
		if here.compare(place(fset.PositionFor(smallest, true))) < 0 {
			smallest = fn.Pos()
		}
	}
	return smallest
}

type place token.Position

func (p place) compare(other place) int {
	return cmp.Or(
		strings.Compare(p.Filename, other.Filename),
		cmp.Compare(p.Line, other.Line),
		cmp.Compare(p.Column, other.Column),
	)
}
