// Package tsidp reproduces tailscale.com@v1.96.5 cmd/tsidp/ui.go:67 (handleClientsList): a
// tie-free comparator ending on ID, where ID equals the map key by invariant.
package tsidp

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"

	"fullcoverage/cases/variant"
)

type funnelClient struct {
	ID, Name, RedirectURI, Secret string
}

type clientDisplayData struct {
	ID, Name, RedirectURI string
	HasSecret             bool
}

// Original is the shipped shape: deterministic, but a comparator sort after a map loop.
func Original(funnelClients map[string]*funnelClient) []clientDisplayData {
	clients := make([]clientDisplayData, 0, len(funnelClients))
	for _, c := range funnelClients {
		clients = append(clients, clientDisplayData{
			ID:          c.ID,
			Name:        c.Name,
			RedirectURI: c.RedirectURI,
			HasSecret:   c.Secret != "",
		})
	}

	sort.Slice(clients, func(i, j int) bool {
		if clients[i].Name != clients[j].Name {
			return clients[i].Name < clients[j].Name
		}
		return clients[i].ID < clients[j].ID
	})
	return clients
}

// S1 ranges the sorted keys and keeps the comparator as written.
func S1(funnelClients map[string]*funnelClient) []clientDisplayData {
	clients := make([]clientDisplayData, 0, len(funnelClients))
	for _, id := range slices.Sorted(maps.Keys(funnelClients)) {
		c := funnelClients[id]
		clients = append(clients, clientDisplayData{
			ID:          c.ID,
			Name:        c.Name,
			RedirectURI: c.RedirectURI,
			HasSecret:   c.Secret != "",
		})
	}

	sort.Slice(clients, func(i, j int) bool {
		if clients[i].Name != clients[j].Name {
			return clients[i].Name < clients[j].Name
		}
		return clients[i].ID < clients[j].ID
	})
	return clients
}

// S1Stable ranges the sorted keys and drops the now-redundant ID tie-break.
func S1Stable(funnelClients map[string]*funnelClient) []clientDisplayData {
	clients := make([]clientDisplayData, 0, len(funnelClients))
	for _, id := range slices.Sorted(maps.Keys(funnelClients)) {
		c := funnelClients[id]
		clients = append(clients, clientDisplayData{
			ID:          c.ID,
			Name:        c.Name,
			RedirectURI: c.RedirectURI,
			HasSecret:   c.Secret != "",
		})
	}

	slices.SortStableFunc(clients, func(a, b clientDisplayData) int { return cmp.Compare(a.Name, b.Name) })
	return clients
}

// S4 rewrites the comparator with cmp.Or; the map loop still fires.
func S4(funnelClients map[string]*funnelClient) []clientDisplayData {
	clients := make([]clientDisplayData, 0, len(funnelClients))
	for _, c := range funnelClients {
		clients = append(clients, clientDisplayData{
			ID:          c.ID,
			Name:        c.Name,
			RedirectURI: c.RedirectURI,
			HasSecret:   c.Secret != "",
		})
	}

	slices.SortFunc(clients, func(a, b clientDisplayData) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return clients
}

// S6 sorts the values directly; maps.Values is not a map, so TS-T02 never sees it.
func S6(funnelClients map[string]*funnelClient) []clientDisplayData {
	clients := make([]clientDisplayData, 0, len(funnelClients))
	byName := func(a, b *funnelClient) int { return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID)) }
	for _, c := range slices.SortedFunc(maps.Values(funnelClients), byName) {
		clients = append(clients, clientDisplayData{
			ID:          c.ID,
			Name:        c.Name,
			RedirectURI: c.RedirectURI,
			HasSecret:   c.Secret != "",
		})
	}
	return clients
}

func fixture() map[string]*funnelClient {
	m := map[string]*funnelClient{}
	for i := range 10 {
		id := fmt.Sprintf("c%02d", i)
		m[id] = &funnelClient{ID: id, Name: fmt.Sprintf("app%d", i%3)}
	}
	return m
}

func render(f func(map[string]*funnelClient) []clientDisplayData) func() string {
	return func() string { return fmt.Sprint(f(fixture())) }
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "S1", Run: render(S1)},
	{Name: "S1Stable", Run: render(S1Stable)},
	{Name: "S4", Run: render(S4)},
	{Name: "S6", Run: render(S6)},
}
