package tsk03

// FromSingleMakeInterface calls through a receiver built from one
// MakeInterface of one concrete type — the exact shape closed-dispatch
// asks for.
func FromSingleMakeInterface() {
	var s Storage = diskStorage{}
	s.Write("x")
}

// FromAddressableLocal calls through a receiver whose address is taken
// (forcing SSA to keep it as a stack slot instead of a plain register),
// stored to exactly once through that pointer — the Store/Load pair
// closedworld's backward walk resolves.
func FromAddressableLocal() {
	var s Storage
	p := &s
	*p = diskStorage{}
	s.Write("x")
}

// FromSameTypePhi builds the receiver from the same concrete type on
// both branches — the Phi node's two edges agree, so it still
// devirtualizes to one type.
func FromSameTypePhi(pick bool) {
	var s Storage
	if pick {
		s = diskStorage{}
	} else {
		s = diskStorage{}
	}
	s.Write("x")
}
