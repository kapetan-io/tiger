package revision

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// maxEntries bounds how many archive entries Extract writes, so a corrupt
// archive cannot fill the disk.
const maxEntries = 10_000_000

// Tree is the repository as it was at the merge base, written outside the
// repository.
type Tree struct {
	// Dir is where the run directory would be inside the extracted tree.
	Dir string
	// Present is false when the run directory did not exist at the merge
	// base, or no go.mod above it did.
	Present bool
	root    string
}

// Close removes the extracted tree.
func (t Tree) Close() error {
	return os.RemoveAll(t.root)
}

// Extract writes the whole repository at the merge base into a new temporary
// directory with git archive, because go.mod, go.work and replace targets can
// sit above the run directory. Files git ignores are not in it.
func (c Comparison) Extract() (Tree, error) {
	root, err := os.MkdirTemp("", "tiger-changed-*")
	if err != nil {
		return Tree{}, err
	}
	if err := c.write(root); err != nil {
		return Tree{}, errors.Join(err, os.RemoveAll(root))
	}
	run := filepath.FromSlash(strings.TrimSuffix(c.repo.prefix, "/"))
	tree := Tree{Dir: filepath.Join(root, run), root: root}
	tree.Present = tree.hasModule()
	return tree, nil
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

// climb lists the run directory and each directory above it up to the top of
// the extracted tree. It only does path arithmetic, so its length is the
// number of path elements.
func (t Tree) climb() []string {
	dirs := []string{t.Dir}
	rel, err := filepath.Rel(t.root, t.Dir)
	if err != nil || rel == "." {
		return append(dirs, t.root)[:1]
	}
	up := t.Dir
	for range strings.Count(rel, string(filepath.Separator)) + 1 {
		up = filepath.Dir(up)
		dirs = append(dirs, up)
	}
	return dirs
}

func (c Comparison) write(dest string) error {
	archive := exec.Command("git", "archive", "--format=tar", c.MergeBase)
	archive.Dir = c.repo.root
	var stderr strings.Builder
	archive.Stderr = &stderr
	out, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	if err := archive.Start(); err != nil {
		return fmt.Errorf("git archive: %w", err)
	}
	extractErr := extract(out, dest)
	// Drain what extract left so git can exit, then collect its status.
	_, drainErr := io.Copy(io.Discard, out)
	if err := archive.Wait(); err != nil {
		return fmt.Errorf("git archive %s: %w: %s",
			Short(c.MergeBase), err, strings.TrimSpace(stderr.String()))
	}
	return errors.Join(extractErr, drainErr)
}

// extract unpacks a tar stream under dest. Every name is confined to dest.
func extract(in io.Reader, dest string) (err error) {
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	reader := tar.NewReader(in)
	for range maxEntries {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading the base tree: %w", err)
		}
		name := path.Clean(header.Name)
		if header.Typeflag == tar.TypeXGlobalHeader || name == "." {
			continue
		}
		if err := place(root, reader, header, filepath.FromSlash(name)); err != nil {
			return fmt.Errorf("writing %s of the base tree: %w", header.Name, err)
		}
	}
	return fmt.Errorf("the base tree has more than %d entries", maxEntries)
}

func place(root *os.Root, content io.Reader, header *tar.Header, name string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return root.MkdirAll(name, 0o755)
	case tar.TypeSymlink:
		if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		return root.Symlink(header.Linkname, name)
	case tar.TypeReg:
		if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if header.Mode&0o111 != 0 {
			mode = 0o755
		}
		file, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(file, content)
		return errors.Join(copyErr, file.Close())
	}
	return nil
}
