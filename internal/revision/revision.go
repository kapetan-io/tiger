// Package revision resolves the two sides of a tiger changed run, the merge
// base of a base ref and HEAD against the working tree, and writes the base
// tree into a temporary directory. It is the only package that runs git, so
// tiger check stays free of git (ADR-0022).
package revision

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const shortLength = 7

type Repository struct {
	dir    string
	root   string
	prefix string
}

type Comparison struct {
	BaseRef   string
	BaseTip   string
	MergeBase string
	Head      string
	Dirty     bool
	repo      Repository
}

func Open(dir string) (Repository, error) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return Repository{}, fmt.Errorf("cannot read directory %s", dir)
	}
	repo := Repository{dir: dir}
	prefix, err := repo.git("rev-parse", "--show-prefix")
	if err != nil {
		return Repository{}, notRepository(err)
	}
	root, err := repo.git("rev-parse", "--show-toplevel")
	if err != nil {
		return Repository{}, notRepository(err)
	}
	repo.prefix, repo.root = prefix, root
	return repo, nil
}

func notRepository(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return errors.New("this command needs the git program, and it is not installed")
	}
	return errors.New("run it inside a git checkout, because it compares against a git revision")
}

func (r Repository) Compare(base string) (Comparison, error) {
	if strings.HasPrefix(base, "-") {
		return Comparison{}, fmt.Errorf(
			"revision %q starts with a dash, so git would read it as an option", base)
	}
	tip, err := r.commit(base)
	if err != nil {
		return Comparison{}, fmt.Errorf("cannot find revision %s", base)
	}
	head, err := r.commit("HEAD")
	if err != nil {
		return Comparison{}, errors.New(
			"this repository has no commits yet, so there is nothing to compare against")
	}
	mergeBase, err := r.git("merge-base", head, tip)
	if err != nil {
		return Comparison{}, fmt.Errorf(
			"no common ancestor of %s and HEAD; in CI fetch full history (fetch-depth: 0)", base)
	}
	status, err := r.git("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return Comparison{}, fmt.Errorf("git status failed: %w", err)
	}
	return Comparison{
		BaseRef:   base,
		BaseTip:   tip,
		MergeBase: mergeBase,
		Head:      head,
		Dirty:     status != "",
		repo:      r,
	}, nil
}

func (r Repository) commit(ref string) (string, error) {
	return r.git("rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
}

func Short(hash string) string {
	if len(hash) > shortLength {
		return hash[:shortLength]
	}
	return hash
}

func (r Repository) git(args ...string) (string, error) {
	return r.gitEnv(nil, args...)
}

var locationVars = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_PREFIX",
	"GIT_NAMESPACE", "GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM",
}

func cleanEnviron() []string {
	return slices.DeleteFunc(os.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return slices.Contains(locationVars, name)
	})
}

type environ []string

func (r Repository) gitEnv(env environ, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(append(cleanEnviron(), "GIT_OPTIONAL_LOCKS=0"), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", err
		}
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}
