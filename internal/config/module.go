package config

import (
	"errors"
	"fmt"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

// Module is a module path, as read from go.mod.
type Module string

// Relative rewrites pkgPath relative to the module: the module root is
// ".", a package below it loses the module prefix, and a package outside
// the module returns ok false.
func (m Module) Relative(pkgPath string) (string, bool) {
	if pkgPath == string(m) {
		return ".", true
	}
	if rest, found := strings.CutPrefix(pkgPath, string(m)+"/"); found {
		return rest, true
	}
	return "", false
}

// ModulePath reads the module path from dir's go.mod. The go.mod parser
// reads the line, so a trailing comment or a quoted path is not taken for
// part of the path.
func ModulePath(dir string) (Module, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading module path: %w", err)
	}
	module := modfile.ModulePath(raw)
	if module == "" {
		return "", errors.New("go.mod has no module line")
	}
	return Module(module), nil
}

// Package is one package directory of a module.
type Package struct {
	// Dir is the package's directory relative to the module root,
	// slash-separated: "." for the root, "internal/store" below it.
	Dir string
	// File is the package's first non-test Go file, as a slash-separated
	// path relative to the module root.
	File string
}

// Layout is what one walk of a module's directories finds.
type Layout struct {
	// Packages are the module's package directories, sorted by Dir.
	Packages []Package
	// Nested are the paths of the modules nested below the root: directories
	// with a go.mod of their own, whose packages belong to that module.
	Nested []Module
}

// ReadLayout walks the module rooted at dir. A directory is a package when
// it holds a non-test Go file, whatever its GOOS or GOARCH constraints say,
// so the answer is the same on every platform. A file marked
// //go:build ignore is the exception, because no platform builds it. Hidden
// and underscore directories, testdata, vendor, and nested modules are
// skipped, as in go list ./... .
func ReadLayout(dir string) (Layout, error) {
	// WalkDir does not follow a symlinked root, so a -C through a symlink
	// would otherwise find no packages at all.
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return Layout{}, fmt.Errorf("listing packages: %w", err)
	}
	layout := Layout{Packages: []Package{}, Nested: []Module{}}
	seen := map[string]bool{}
	err = filepath.WalkDir(root, func(walked string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && walked != root {
			return layout.enter(walked, entry)
		}
		pkgDir := filepath.Dir(walked)
		if !isSource(entry.Name()) || seen[pkgDir] || neverBuilt(walked) {
			return nil
		}
		seen[pkgDir] = true
		rel, err := filepath.Rel(root, walked)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		layout.Packages = append(layout.Packages, Package{Dir: path.Dir(rel), File: rel})
		return nil
	})
	if err != nil {
		return Layout{}, fmt.Errorf("listing packages: %w", err)
	}
	slices.SortFunc(layout.Packages, func(a, b Package) int {
		return strings.Compare(a.Dir, b.Dir)
	})
	return layout, nil
}

// enter returns filepath.SkipDir for a directory below the module root that
// the go command skips for ./..., recording a nested module's path, and nil
// for one it enters. Only a go.mod file starts a module, not a directory
// that happens to be named go.mod.
func (l *Layout) enter(walked string, entry fs.DirEntry) error {
	name := entry.Name()
	if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") ||
		strings.HasPrefix(name, "_") {
		return filepath.SkipDir
	}
	info, err := os.Stat(filepath.Join(walked, "go.mod"))
	if err != nil || info.IsDir() {
		return nil
	}
	// A go.mod with no module line names no import path to keep out.
	if nested, err := ModulePath(walked); err == nil {
		l.Nested = append(l.Nested, nested)
	}
	return filepath.SkipDir
}

// isSource reports whether a file name is a Go file that belongs to a
// package: not a test, and not hidden from the go tool.
func isSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") &&
		!strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_")
}

// neverBuilt reports whether the Go file's build constraint is exactly the
// tag ignore, the convention for a "go run" script that no build includes.
// Other constraints are left as packages, because a file tiger can't prove
// unbuildable must keep its entry. The legacy // +build form is not read:
// its placement rules differ from //go:build, and gofmt has added the
// //go:build twin to such files since Go 1.17.
func neverBuilt(file string) bool {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil,
		parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return false
	}
	for _, group := range parsed.Comments {
		if group.Pos() > parsed.Package {
			break
		}
		for _, comment := range group.List {
			if !constraint.IsGoBuild(comment.Text) {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err != nil {
				return false
			}
			tag, ok := expr.(*constraint.TagExpr)
			return ok && tag.Tag == "ignore"
		}
	}
	return false
}
