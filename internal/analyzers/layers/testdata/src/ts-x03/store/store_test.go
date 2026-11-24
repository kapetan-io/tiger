package store

// Test files are not layered: net/http needs no entry here.
import (
	_ "net/http"
	"path/filepath"
)

func scan() { _, _ = filepath.Glob("*") }
