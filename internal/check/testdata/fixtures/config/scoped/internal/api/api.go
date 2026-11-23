// Package api is inside the tiger.yaml entry's package scope, so the
// per-item storage call fires TS-M10 here.
package api

import "fixture.example/scoped/storage"

func SaveAll(items []string) error {
	for _, item := range items {
		if err := storage.Save(item); err != nil {
			return err
		}
	}
	return nil
}
