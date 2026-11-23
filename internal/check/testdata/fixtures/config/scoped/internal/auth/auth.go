// Package auth is outside the tiger.yaml entry's package scope, so the same
// per-item storage call stays silent here.
package auth

import "fixture.example/scoped/storage"

func SaveAll(items []string) error {
	for _, item := range items {
		if err := storage.Save(item); err != nil {
			return err
		}
	}
	return nil
}
