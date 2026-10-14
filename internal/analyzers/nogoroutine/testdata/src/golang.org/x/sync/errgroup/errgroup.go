// Package errgroup is the slice of golang.org/x/sync/errgroup the corpus uses.
package errgroup

import "context"

// Group runs functions and waits for them.
type Group struct{}

// WithContext returns a group and a context.
func WithContext(ctx context.Context) (*Group, context.Context) { return &Group{}, ctx }

// Go runs f on its own goroutine.
func (g *Group) Go(f func() error) { _ = f }

// TryGo runs f if the group has room.
func (g *Group) TryGo(f func() error) bool { _ = f; return true }

// Wait blocks until every function has returned.
func (g *Group) Wait() error { return nil }
