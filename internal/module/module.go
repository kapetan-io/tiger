// Package module owns the facts about the Go module a run sits in: its path
// from go.mod, module-relative package paths, the directory layout and the
// Go files, from one walk of its tree. It does not read tiger.yaml.
package module

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

type Path string

func (m Path) Relative(pkgPath string) (string, bool) {
	if pkgPath == string(m) {
		return ".", true
	}
	if rest, found := strings.CutPrefix(pkgPath, string(m)+"/"); found {
		return rest, true
	}
	return "", false
}

func ReadPath(dir string) (Path, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading module path: %w", err)
	}
	found := modfile.ModulePath(raw)
	if found == "" {
		return "", errors.New("go.mod has no module line")
	}
	return Path(found), nil
}

type Package struct {
	Dir  string
	File string
}

type Layout struct {
	Packages []Package
	Nested   []Path
}

func ReadLayout(dir string) (Layout, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return Layout{}, fmt.Errorf("listing packages: %w", err)
	}
	layout := Layout{Packages: []Package{}, Nested: []Path{}}
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
	if nested, err := ReadPath(walked); err == nil {
		l.Nested = append(l.Nested, nested)
	}
	return filepath.SkipDir
}

func isSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") &&
		!strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_")
}

func GoFiles(dir string) (map[string][]string, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("listing Go files: %w", err)
	}
	var layout Layout
	files := map[string][]string{}
	err = filepath.WalkDir(root, func(walked string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && walked != root {
			return layout.enter(walked, entry)
		}
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, ".") ||
			strings.HasPrefix(name, "_") || neverBuilt(walked) {
			return nil
		}
		rel, err := filepath.Rel(root, walked)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files[path.Dir(rel)] = append(files[path.Dir(rel)], rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing Go files: %w", err)
	}
	return files, nil
}

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
