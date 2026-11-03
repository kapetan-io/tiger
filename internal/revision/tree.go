package revision

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Tree struct {
	Dir     string
	Present bool
	top     string
	root    string
}

func (t Tree) Close() error {
	return os.RemoveAll(t.root)
}

func (c Comparison) Extract() (Tree, error) {
	root, err := os.MkdirTemp("", "tiger-changed-*")
	if err != nil {
		return Tree{}, err
	}
	top := filepath.Join(root, "tree")
	if err := c.write(root); err != nil {
		return Tree{}, errors.Join(err, os.RemoveAll(root))
	}
	run := filepath.FromSlash(strings.TrimSuffix(c.repo.prefix, "/"))
	tree := Tree{Dir: filepath.Join(top, run), top: top, root: root}
	tree.Present = tree.hasModule()
	return tree, nil
}

func (c Comparison) write(scratch string) error {
	top := filepath.Join(scratch, "tree")
	env := environ{"GIT_INDEX_FILE=" + filepath.Join(scratch, "index")}
	if err := os.Mkdir(top, 0o755); err != nil {
		return err
	}
	repo := c.repo
	repo.dir = repo.root
	if _, err := repo.gitEnv(env, "read-tree", c.MergeBase); err != nil {
		return err
	}
	empty, err := repo.git("hash-object", "-t", "tree", "/dev/null")
	if err != nil {
		return err
	}
	_, err = repo.gitEnv(env, "--attr-source="+empty, "checkout-index", "--all", "--quiet",
		"--prefix="+top+"/")
	return err
}

func (t Tree) hasModule() bool {
	if info, err := os.Stat(t.Dir); err != nil || !info.IsDir() {
		return false
	}
	return slices.ContainsFunc(t.climb(), func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		return err == nil
	})
}

func (t Tree) climb() []string {
	dirs := []string{t.Dir}
	rel, err := filepath.Rel(t.top, t.Dir)
	if err != nil || rel == "." {
		return dirs
	}
	up := t.Dir
	for range strings.Count(rel, string(filepath.Separator)) + 1 {
		up = filepath.Dir(up)
		dirs = append(dirs, up)
	}
	return dirs
}
