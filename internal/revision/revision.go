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

// shortLength is how many characters of a commit hash a report prints.
const shortLength = 7

// Repository is the git checkout holding a run directory.
type Repository struct {
	dir string
	// root is the repository's top directory.
	root string
	// prefix is the run directory's path below the root, with a trailing
	// slash, empty at the root.
	prefix string
}

// Comparison names the two sides of a run.
type Comparison struct {
	// BaseRef is the ref the user typed.
	BaseRef string
	// BaseTip is the full hash BaseRef points at.
	BaseTip string
	// MergeBase is the full hash of the common ancestor of BaseTip and Head.
	// Base code is read from this commit.
	MergeBase string
	// Head is the full hash of HEAD. The head code is the working tree, which
	// may carry uncommitted edits.
	Head string
	// Dirty is true when git status prints anything.
	Dirty bool
	repo  Repository
}

// Open finds the repository holding dir. Every error is a single line written
// for someone who has never seen tiger.
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

// Compare finds the merge base of base and HEAD. It never changes the working
// tree, the index or the stash.
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

// commit resolves a ref to a full commit hash. The ref comes after
// --end-of-options so git never reads it as an option.
func (r Repository) commit(ref string) (string, error) {
	return r.git("rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
}

// Short is the first characters of a commit hash, as a report prints them.
func Short(hash string) string {
	if len(hash) > shortLength {
		return hash[:shortLength]
	}
	return hash
}

// git runs one git command in the run directory and returns its trimmed
// standard output. GIT_OPTIONAL_LOCKS stops git status from refreshing the
// index on disk.
func (r Repository) git(args ...string) (string, error) {
	return r.gitEnv(nil, args...)
}

// locationVars are the variables that point git at a repository other than the
// one in the working directory. Git sets them when it runs a hook.
var locationVars = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_PREFIX",
	"GIT_NAMESPACE", "GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM",
}

// cleanEnviron is the process environment without the repository-location
// variables, so a run inside a hook reads the -C directory.
func cleanEnviron() []string {
	return slices.DeleteFunc(os.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return slices.Contains(locationVars, name)
	})
}

// environ is extra environment entries for a git command.
type environ []string

// gitEnv is git with extra environment entries, such as a scratch index file.
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
