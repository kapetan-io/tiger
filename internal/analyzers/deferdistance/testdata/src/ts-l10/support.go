// Package fixture provides small acquire/release helpers shared across the
// TS-L10 corpus so each case can focus on defer placement.
package fixture

type resource struct{}

func (r *resource) Close() error { return nil }

func openResource() (*resource, error) {
	return &resource{}, nil
}

type transaction struct{}

func (t *transaction) Begin() {}

func (t *transaction) Rollback() error { return nil }

func beginTransaction() (*transaction, error) {
	return &transaction{}, nil
}

type gate struct{}

func (g *gate) Lock() {}

func (g *gate) Unlock() {}

var sharedGate gate
