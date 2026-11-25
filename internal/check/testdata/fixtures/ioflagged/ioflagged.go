// Package ioflagged proves the -ioinloop.packages flag adds a package to
// what TS-M10 counts as IO through the tiger check driver: SaveAll's per-item call is
// ordinary local code by default (no tiger rule fires), and only becomes a
// TS-M10 finding once its import path is named on the flag.
package ioflagged

import "fixture.example/ioflagged/storage"


func SaveAll(items []string) error {
	for _, item := range items {
		if err := storage.Save(item); err != nil {
			return err
		}
	}
	return nil
}
