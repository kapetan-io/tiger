package layerfile

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type standardLibrary struct {
	goroot string
	mu     sync.Mutex
	known  map[string]bool
}

func newStandardLibrary(dir string) *standardLibrary {
	return &standardLibrary{goroot: goroots.lookup(dir), known: map[string]bool{}}
}

func (s *standardLibrary) contains(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	if s.goroot == "" {
		return !strings.Contains(first, ".")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if known, found := s.known[importPath]; found {
		return known
	}
	info, err := os.Stat(filepath.Join(s.goroot, "src", filepath.FromSlash(importPath)))
	s.known[importPath] = err == nil && info.IsDir() && first != "vendor"
	return s.known[importPath]
}

type gorootCache struct {
	mu    sync.Mutex
	byDir map[string]string
}

var goroots = &gorootCache{byDir: map[string]string{}}

func (c *gorootCache) lookup(dir string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if root, found := c.byDir[dir]; found {
		return root
	}
	root := ""
	command := exec.Command("go", "env", "GOROOT")
	command.Dir = dir
	if out, err := command.Output(); err == nil {
		root = strings.TrimSpace(string(out))
	}
	if !holdsStandardLibrary(root) {
		root = build.Default.GOROOT
	}
	if !holdsStandardLibrary(root) {
		root = ""
	}
	c.byDir[dir] = root
	return root
}

func holdsStandardLibrary(root string) bool {
	if root == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(root, "src", "runtime"))
	return err == nil && info.IsDir()
}
