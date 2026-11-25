// The TS-S01 failure modes: direct self-recursion, mutual recursion, method
// recursion on a concrete type, and recursion through a statically-known
// function literal. The corpus also holds the same recursion on generic
// types and through a method value or method expression.
package fixture

// Node is one node of a tree walked by chase and chaseIterative.
type Node struct {
	Children []*Node
}

// chase walks the tree depth-first by calling itself on every child — its
// stack depth is bounded only by the tree's depth, which comes from data.
func chase(n *Node, visit func(*Node)) { // want `TS-S01: chase → chase`
	if n == nil {
		return
	}
	visit(n)
	for _, child := range n.Children {
		chase(child, visit)
	}
}

// Even and Odd decide parity by calling each other down to zero — mutual
// recursion, still a cycle in the static call graph.
func Even(n int) bool { // want `TS-S01: Even → Odd → Even`
	if n == 0 {
		return true
	}
	return Odd(n - 1)
}

func Odd(n int) bool {
	if n == 0 {
		return false
	}
	return Even(n - 1)
}

// List is a linked list walked by Sum and SumIterative.
type List struct {
	val  int
	next *List
}

// Sum recurses down the linked list one node at a time — a method calling
// itself on a concrete receiver is still a self-edge.
func (l *List) Sum() int { // want `TS-S01: Sum → Sum`
	if l == nil {
		return 0
	}
	return l.val + l.next.Sum()
}

// runA calls itself through a directly-invoked local closure: helper is
// never stored anywhere its target could be ambiguous, so the call through
// it is statically known, and the cycle it forms is real.
func runA() { // want `TS-S01: runA → runA\$1 → runA`
	helper := func() {
		runA()
	}
	helper()
}

// genSelf calls itself with a type parameter. The call targets an instance,
// a different function object from genSelf.
func genSelf[T any](x T) { // want `TS-S01: genSelf → genSelf`
	genSelf(x)
}

// genPing and genPong are a generic mutual cycle.
func genPing[T any](x T) { // want `TS-S01: genPing → genPong → genPing`
	genPong(x)
}

func genPong[T any](x T) {
	genPing(x)
}

// plainCaller and genCallee form a cycle through an explicit instantiation.
// The finding lists members in name order, so genCallee comes first.
func plainCaller() { // want `TS-S01: genCallee → plainCaller → genCallee`
	genCallee[int](1)
}

func genCallee[T any](x T) {
	plainCaller()
}

// GenBox is a generic type whose method recurses.
type GenBox[T any] struct{ next *GenBox[T] }

func (b *GenBox[T]) Depth() int { // want `TS-S01: Depth → Depth`
	if b == nil {
		return 0
	}
	return 1 + b.next.Depth()
}

// genClosure reaches itself through a closure inside a generic function.
func genClosure[T any](x T) { // want `TS-S01: genClosure → genClosure\$1 → genClosure`
	helper := func() { genClosure(x) }
	helper()
}

// genValue calls its own instance through a function value.
func genValue[T any](x T) { // want `TS-S01: genValue → genValue`
	next := genValue[int]
	next(1)
}

// Nest is a generic type whose method calls itself at another instantiation.
type Nest[T any] struct{}

func (n Nest[T]) Down(depth int) { // want `TS-S01: Down → Down`
	Nest[int]{}.Down(depth - 1)
}

// MV is a plain type whose method recurses through a method value.
type MV struct{ next *MV }

func (b *MV) Depth() int { // want `TS-S01: Depth → Depth`
	f := b.next.Depth
	return f()
}

// GenMV is the same shape on a generic type.
type GenMV[T any] struct{ next *GenMV[T] }

func (b *GenMV[T]) Depth() int { // want `TS-S01: Depth → Depth`
	f := b.next.Depth
	return f()
}

// MVPair.Ping and MVPair.Pong recurse through a method value across two methods.
type MVPair struct{ other *MVPair }

func (p *MVPair) Ping() { // want `TS-S01: Ping → Pong → Ping`
	f := p.other.Pong
	f()
}

func (p *MVPair) Pong() {
	p.other.Ping()
}

type ME struct{}

func (m *ME) Go(n int) { // want `TS-S01: Go → Go`
	f := (*ME).Go
	f(m, n-1)
}

// GenME is the method-expression shape on a generic type.
type GenME[T any] struct{}

func (m *GenME[T]) Go(n int) { // want `TS-S01: Go → Go`
	f := (*GenME[T]).Go
	f(m, n-1)
}
