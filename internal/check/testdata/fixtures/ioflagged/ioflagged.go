// Package ioflagged proves the -ioinloop.packages flag extends the seed IO
// allowlist through the tiger check driver: SaveAll's per-item call is
// ordinary local code by default (no tiger rule fires), and only becomes a
// TS-M10 finding once its import path is named on the flag.
package ioflagged

import "fixture.example/ioflagged/storage"

// SaveAll writes every item in the batch.
func SaveAll(items []string) error {
	for _, item := range items {
		if err := storage.Save(item); err != nil {
			return err
		}
	}
	return nil
}
