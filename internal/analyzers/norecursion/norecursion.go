// Package norecursion enforces TS-S01: no cycle in the package's static call
// graph.
//
// A recursive function's stack depth depends on data, and data comes from
// outside; Go grows stacks to 1GB by default, so an unbounded recursion does
// not fail fast, it quietly eats memory and dies far from the cause. The
// compliant rewrite bounds the depth explicitly: a stack or queue driven by
// an index-advancing worklist, or an iteration cap with an assert on
// exhaustion.
//
// Deliberate deviation from spec text. TS-S01's enforcement text says "CHA
// callgraph." CHA (class hierarchy analysis) resolves an interface call to
// every implementing type, so it reports cycles that can never occur at
// runtime — a blocking false positive, which this wave's correctness
// constraint 7 defines as a bug in the analyzer, not the code. This analyzer
// builds the static call graph instead, from ssalib.StaticCalls: direct
// calls, concrete-method calls, and calls through a function literal whose
// callee is immediately known. Every edge in that graph is a call that
// provably happens, so every reported cycle is real.
//
// Known-miss boundary. Three call shapes fall outside the static call graph
// and so outside this analyzer's reach: recursion through an interface
// method (the receiver's dynamic type decides the callee), recursion through
// a function value whose target is not statically known (assigned behind a
// branch or a Phi), and recursion whose cycle spans package boundaries (this
// analyzer's node set is one package's functions; the facts plumbing a later
// wave builds is the natural way to close this gap). All three are marked
// known-miss in the corpus rather than reported, matching Undecidability
// resolves to a known-miss, never a finding. Flagged for spec amendment
// alongside the wave-1 TS-S02 amendment.
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
)

const msgRewrite = ": rewrite the recursion as an explicit stack or queue with a bound: an " +
	"index-advancing worklist (for i := 0; i < len(work); i++ { work = append(work, next...) }) " +
	"or an iteration cap with an assert on exhaustion"

// Analyzer enforces TS-S01: no cycle in the package's static call graph.
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
			called = append(called, call.Callee)
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

// collectFunctions gathers every function reachable from sourceFuncs by
// walking AnonFuncs, so a cycle through a statically-called literal has both
// of its members as graph nodes. The worklist advances by index over a
// growing slice; a seen set keeps a function that already carries its own
// anonymous children from being re-queued.
func collectFunctions(sourceFuncs []*ssa.Function) []*ssa.Function {
	seen := map[*ssa.Function]bool{}
	collected := []*ssa.Function{}
	for _, fn := range sourceFuncs {
		if !seen[fn] {
			seen[fn] = true
			collected = append(collected, fn)
		}
	}
	for i := 0; i < len(collected); i++ {
		for _, anon := range collected[i].AnonFuncs {
			if !seen[anon] {
				seen[anon] = true
				collected = append(collected, anon)
			}
		}
	}
	return collected
}

// isCycle reports whether component is a real cycle: more than one member,
// or a single member that statically calls itself.
func isCycle(component []*ssa.Function, callees map[*ssa.Function][]*ssa.Function) bool {
	if len(component) > 1 {
		return true
	}
	self := component[0]
	return slices.Contains(callees[self], self)
}

// report fires one TS-S01 finding for component, at the member with the
// smallest source position, naming every member sorted by name.
func report(pass *analysis.Pass, component []*ssa.Function) {
	sorted := sortedByName(component)
	names := make([]string, 0, len(sorted)+1)
	for _, fn := range sorted {
		names = append(names, fn.Name())
	}
	names = append(names, sorted[0].Name())

	pass.Report(analysis.Diagnostic{
		Pos:      firstPos(component),
		Category: "TS-S01",
		Message:  "TS-S01: " + strings.Join(names, " → ") + msgRewrite,
	})
}

// sortedByName returns component's members ordered by name, breaking a name
// collision (two methods sharing an unqualified name) by source position so
// the order stays deterministic.
func sortedByName(component []*ssa.Function) []*ssa.Function {
	sorted := slices.Clone(component)
	slices.SortFunc(sorted, func(a, b *ssa.Function) int {
		if a.Name() != b.Name() {
			return strings.Compare(a.Name(), b.Name())
		}
		return cmp.Compare(a.Pos(), b.Pos())
	})
	return sorted
}

// firstPos returns the smallest source position among component's members —
// the cycle's lexically first member, and where the finding is reported.
func firstPos(component []*ssa.Function) token.Pos {
	smallest := component[0].Pos()
	for _, fn := range component[1:] {
		if fn.Pos() < smallest {
			smallest = fn.Pos()
		}
	}
	return smallest
}
