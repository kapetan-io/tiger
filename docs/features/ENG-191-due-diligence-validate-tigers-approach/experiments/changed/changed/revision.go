package changed

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LoadRevision computes the edges of the module in dir as it was at rev.
// It extracts rev with git archive into a temporary directory, so the
// repository's working tree, index, stash and worktree list are untouched.
// Gitignored files, such as generated code, are not in the archive.
func LoadRevision(dir, rev string, filter Filter) (Edges, error) {
	root, err := toplevel(dir)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	sub, err := filepath.Rel(root, abs)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "changed-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	archive := exec.Command("git", "archive", "--format=tar", rev)
	archive.Dir = root
	extract := exec.Command("tar", "-x", "-C", tmp)
	extract.Stdin, err = archive.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	archive.Stderr = &stderr
	if err := extract.Start(); err != nil {
		return nil, err
	}
	if err := archive.Run(); err != nil {
		return nil, fmt.Errorf("git archive %s: %w: %s", rev, err, stderr.String())
	}
	if err := extract.Wait(); err != nil {
		return nil, fmt.Errorf("tar: %w", err)
	}
	return Load(filepath.Join(tmp, sub), filter)
}

// toplevel returns the repository root holding dir, symlinks resolved so
// it can be compared with dir.
func toplevel(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w: %s", err, out)
	}
	path, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return "", err
	}
	return path, nil
}
