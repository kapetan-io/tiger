package ssalib

import (
	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/assert"
)

// walkFrame is one suspended node in the iterative Tarjan walk: the node
// and how many of its edges have been explored.
type walkFrame struct {
	fn   *ssa.Function
	edge int
}

// walkState carries the Tarjan bookkeeping across the iterative walk.
type walkState struct {
	adjacency  map[*ssa.Function][]*ssa.Function
	index      map[*ssa.Function]int
	lowlink    map[*ssa.Function]int
	onStack    map[*ssa.Function]bool
	stack      []*ssa.Function
	frames     []walkFrame
	counter    int
	components [][]*ssa.Function
}

// Components partitions fns into strongly connected components of the
// graph callees induces, in reverse topological order: a component's
// callees complete before the component itself. Edges leaving the node set
// are ignored. The walk is iterative Tarjan — deterministic in the order
// of fns and each node's edge list — and never recurses.
func Components(
	fns []*ssa.Function,
	callees func(*ssa.Function) []*ssa.Function,
) [][]*ssa.Function {
	walk := &walkState{
		adjacency: map[*ssa.Function][]*ssa.Function{},
		index:     map[*ssa.Function]int{},
		lowlink:   map[*ssa.Function]int{},
		onStack:   map[*ssa.Function]bool{},
	}
	members := map[*ssa.Function]bool{}
	for _, fn := range fns {
		members[fn] = true
	}
	edges := 0
	for _, fn := range fns {
		kept := []*ssa.Function{}
		for _, callee := range callees(fn) {
			if members[callee] {
				kept = append(kept, callee)
			}
		}
		walk.adjacency[fn] = kept
		edges += len(kept)
	}
	// Every node is pushed once and every edge examined once; the bound
	// converts a bookkeeping bug into a loud failure instead of a hang.
	steps := 2 + 2*len(fns) + 2*edges
	for _, root := range fns {
		if _, seen := walk.index[root]; seen {
			continue
		}
		walk.push(root)
		for step := 0; step < steps; step++ {
			if len(walk.frames) == 0 {
				break
			}
			walk.advance()
		}
		assert.Ok(len(walk.frames) == 0, "SCC walk exhausted its step bound")
	}
	return walk.components
}

// push starts visiting a node.
func (w *walkState) push(fn *ssa.Function) {
	w.index[fn] = w.counter
	w.lowlink[fn] = w.counter
	w.counter++
	w.stack = append(w.stack, fn)
	w.onStack[fn] = true
	w.frames = append(w.frames, walkFrame{fn: fn})
}

// advance takes one step: explore the top frame's next edge, or finish the
// frame when its edges are exhausted.
func (w *walkState) advance() {
	top := &w.frames[len(w.frames)-1]
	if top.edge < len(w.adjacency[top.fn]) {
		next := w.adjacency[top.fn][top.edge]
		top.edge++
		if _, seen := w.index[next]; !seen {
			w.push(next)
			return
		}
		if w.onStack[next] {
			w.lowlink[top.fn] = min(w.lowlink[top.fn], w.index[next])
		}
		return
	}
	finished := top.fn
	w.frames = w.frames[:len(w.frames)-1]
	if w.lowlink[finished] == w.index[finished] {
		w.pop(finished)
	}
	if len(w.frames) > 0 {
		parent := w.frames[len(w.frames)-1].fn
		w.lowlink[parent] = min(w.lowlink[parent], w.lowlink[finished])
	}
}

// pop collects the finished component rooted at fn off the stack.
func (w *walkState) pop(fn *ssa.Function) {
	component := []*ssa.Function{}
	for range w.stack {
		popped := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		w.onStack[popped] = false
		component = append(component, popped)
		if popped == fn {
			break
		}
	}
	w.components = append(w.components, component)
}
