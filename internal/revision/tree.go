package revision

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Tree is the repository as it was at the merge base, written outside the
// repository.
type Tree struct {
	// Dir is where the run directory would be inside the extracted tree.
	Dir string
	// Present is false when the run directory did not exist at the merge
	// base, or no go.mod above it did.
	Present bool
	// top is the extracted repository root and root the temporary directory
	// holding it.
	top  string
	root string
}

// Close removes the extracted tree.
func (t Tree) Close() error {
	return os.RemoveAll(t.root)
}

// Extract writes the whole repository at the merge base into a new temporary
// directory, because go.mod, go.work and replace targets can sit above the run
// directory. Files git ignores are not in it. It goes through a scratch index
// and checkout-index, since git archive drops files marked export-ignore in
// .gitattributes and so would show a package as new that existed at base.
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

// write fills top with the merge base. The index lives in scratch, never in
// the repository, so the real index is not touched.
func (c Comparison) write(scratch string) error {
	top := filepath.Join(scratch, "tree")
	env := environ{"GIT_INDEX_FILE=" + filepath.Join(scratch, "index")}
	if err := os.Mkdir(top, 0o755); err != nil {
		return err
	}
	// checkout-index writes only the paths below its working directory, so run
	// it at the repository root.
	repo := c.repo
	repo.dir = repo.root
	if _, err := repo.gitEnv(env, "read-tree", c.MergeBase); err != nil {
		return err
	}
	// An empty attribute source gives raw blobs: the working tree's
	// .gitattributes, filters and line-ending settings describe head, not base.
	empty, err := repo.git("hash-object", "-t", "tree", "/dev/null")
	if err != nil {
		return err
	}
	_, err = repo.gitEnv(env, "--attr-source="+empty, "checkout-index", "--all", "--quiet",
		"--prefix="+top+"/")
	return err
}

// hasModule reports whether the run directory exists and a go.mod sits in it
// or above it inside the extracted tree.
func (t Tree) hasModule() bool {
	if info, err := os.Stat(t.Dir); err != nil || !info.IsDir() {
		return false
	}
	return slices.ContainsFunc(t.climb(), func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		return err == nil
	})
}

// climb lists the run directory and each directory above it, up to the
// extracted repository root. It only does path arithmetic.
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
