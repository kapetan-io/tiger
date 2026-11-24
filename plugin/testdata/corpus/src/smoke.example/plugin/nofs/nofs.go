// Package nofs has an empty entry, so it may not walk the file system.
package nofs

import "path/filepath"

func Reads() ([]string, error) { return filepath.Glob("*") }
