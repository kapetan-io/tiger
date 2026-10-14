package layerfile

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// standardLibrary answers whether an import path is in the standard library,
// looking in the GOROOT the go command uses, once per path for the life of the
// loaded file.
type standardLibrary struct {
	goroot string
	mu     sync.Mutex
	known  map[string]bool
}

func newStandardLibrary(dir string) *standardLibrary {
	return &standardLibrary{goroot: goroots.lookup(dir), known: map[string]bool{}}
}

// contains reports whether importPath is a GOROOT package. A path whose
// first element has no dot may still be a third-party module brought in by
// go.work or replace, so the toolchain's own tree decides, leaving out
// GOROOT's vendored copies of x/ modules. With no GOROOT to look in, a path
// whose first element has no dot is taken for the standard library.
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

// gorootCache remembers each run directory's GOROOT for the life of the
// process: asking the go command starts a process, and a directory's
// toolchain does not change during a run.
type gorootCache struct {
	mu    sync.Mutex
	byDir map[string]string
}

// goroots is the process's cache of run directories' GOROOTs.
var goroots = &gorootCache{byDir: map[string]string{}}

// lookup returns the GOROOT the go command uses in dir, the same one the
// driver's package loading sees. It does not trust the GOROOT this binary was
// built with first, because a -trimpath build or a removed toolchain leaves
// that one empty; it falls back to it only when the go command can't answer,
// and returns "" when neither holds a standard library.
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

// holdsStandardLibrary reports whether root is a GOROOT with sources in it.
func holdsStandardLibrary(root string) bool {
	if root == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(root, "src", "runtime"))
	return err == nil && info.IsDir()
}
