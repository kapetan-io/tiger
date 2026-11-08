package callinventory

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/kapetan-io/tiger/assert"
)

const reachFileMagic = "tiger-reachfile/2"

const reachFileEnd = "end"

const markInterval = time.Hour

type ReachFile struct {
	dir      string
	once     sync.Once
	err      error
	saveErr  error
	mu       sync.Mutex
	others   []*ReachFile
	packages map[string]bool
	reaches  map[string]bool
}

var keyedEnv = []string{
	"GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "GOEXPERIMENT", "GOFLAGS", "GOFIPS140",
	"GOAMD64", "GOARM", "GOARM64", "GO386", "GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64", "GOWASM",
}

type toolchain struct {
	goos   string
	goarch string
	cgo    string
	cache  string
	env    map[string]string
	tiger  string
}

func (t toolchain) key() string {
	parts := []string{reachFileMagic, "tiger=" + t.tiger}
	for _, name := range keyedEnv {
		parts = append(parts, fmt.Sprintf("%s=%q", name, t.env[name]))
	}
	return strings.Join(parts, " ")
}

func (t toolchain) entry() (string, error) {
	if !filepath.IsAbs(t.cache) {
		return "", fmt.Errorf("GOCACHE is %q, not an absolute path", t.cache)
	}
	sum := sha256.Sum256([]byte(t.key()))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(t.cache, name[:2], name+"-d"), nil
}

func goToolchain(dir string) (toolchain, error) {
	cmd := exec.Command("go", append([]string{"env", "-json", "GOCACHE"}, keyedEnv...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if exit := new(exec.ExitError); errors.As(err, &exit) {
		return toolchain{}, fmt.Errorf("go env: %w: %s", err, strings.TrimSpace(string(exit.Stderr)))
	}
	if err != nil {
		return toolchain{}, fmt.Errorf("go env: %w", err)
	}
	var env map[string]string
	if err := json.Unmarshal(out, &env); err != nil {
		return toolchain{}, fmt.Errorf("go env: %w", err)
	}
	found := toolchain{
		goos: env["GOOS"], goarch: env["GOARCH"], cgo: env["CGO_ENABLED"],
		cache: env["GOCACHE"], env: env,
	}
	if found.tiger, err = executableSum(); err != nil {
		return toolchain{}, err
	}
	return found, nil
}

var executableSum = sync.OnceValues(hashExecutable)

func hashExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err = errors.Join(err, file.Close()); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)[:8]), nil
}

func NewReachFile(dir string) *ReachFile {
	return &ReachFile{dir: dir}
}

func (t *ReachFile) ready() error {
	t.once.Do(func() { t.err = t.resolve() })
	return t.err
}

func (t *ReachFile) SaveError() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	errs := []error{t.saveErr}
	for _, other := range t.others {
		errs = append(errs, other.saveErr)
	}
	return errors.Join(errs...)
}

func (t *ReachFile) For(dir string) (*ReachFile, error) {
	mine, err := goToolchain(t.dir)
	if err != nil {
		return nil, err
	}
	theirs, err := goToolchain(dir)
	if err != nil {
		return nil, err
	}
	if mine.key() == theirs.key() {
		return t, nil
	}
	other := NewReachFile(dir)
	t.mu.Lock()
	t.others = append(t.others, other)
	t.mu.Unlock()
	return other, nil
}

func (t *ReachFile) resolve() error {
	chain, err := goToolchain(t.dir)
	if err != nil {
		return err
	}
	path, pathErr := chain.entry()
	if pathErr == nil {
		if data, err := os.ReadFile(path); err == nil && t.parse(string(data), chain) == nil {
			markUsed(path)
			return nil
		}
	}
	var text bytes.Buffer
	if err := writeReachFile(&text, t.dir, chain); err != nil {
		return err
	}
	if err := t.parse(text.String(), chain); err != nil {
		return err
	}
	t.saveErr = pathErr
	if pathErr == nil {
		t.saveErr = writeFileAtomic(path, text.Bytes())
	}
	return nil
}

func markUsed(path string) {
	if info, err := os.Stat(path); err == nil {
		if now := time.Now(); now.Sub(info.ModTime()) >= markInterval {
			if err := os.Chtimes(path, now, now); err != nil {
				return
			}
		}
	}
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	pattern := strings.TrimSuffix(filepath.Base(path), "-d") + "-*-d"
	temp, err := os.CreateTemp(filepath.Dir(path), pattern)
	if err != nil {
		return err
	}
	_, err = temp.Write(data)
	err = errors.Join(err, temp.Close())
	if err == nil {
		err = os.Chmod(temp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(temp.Name(), path)
	}
	if err != nil {
		return errors.Join(err, os.Remove(temp.Name()))
	}
	return nil
}

func (t *ReachFile) parse(text string, chain toolchain) error {
	first, rest, _ := strings.Cut(text, "\n")
	if first != chain.key() {
		return fmt.Errorf("reach file is for %q, not %q", first, chain.key())
	}
	body, complete := strings.CutSuffix(strings.TrimRight(rest, "\n"), "\n"+reachFileEnd)
	if !complete {
		return fmt.Errorf("reach file does not end with %q", reachFileEnd)
	}
	t.packages, t.reaches = map[string]bool{}, map[string]bool{}
	pkg := ""
	for i, line := range strings.Split(body, "\n") {
		switch {
		case line == "":
		case strings.HasPrefix(line, "\t"):
			if pkg == "" {
				return fmt.Errorf("line %d: member before any package", i+2)
			}
			if !memberName(line[1:]) {
				return fmt.Errorf("line %d: %q is not a member name", i+2, line[1:])
			}
			t.reaches[pkg+"."+line[1:]] = true
		default:
			if !importPath(line) {
				return fmt.Errorf("line %d: %q is not a package path", i+2, line)
			}
			pkg = line
			t.packages[pkg] = true
		}
	}
	return nil
}

func importPath(path string) bool {
	for _, r := range path {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("-._~/", r) {
			return false
		}
	}
	return path != ""
}

func memberName(name string) bool {
	for _, part := range strings.Split(name, ".") {
		if !token.IsIdentifier(part) {
			return false
		}
	}
	return true
}

func (t *ReachFile) knows(path string) bool {
	return t.packages[path]
}

func (t *ReachFile) member(fn *ssa.Function) bool {
	if origin := fn.Origin(); origin != nil {
		fn = origin
	}
	obj, ok := fn.Object().(*types.Func)
	if !ok || obj.Pkg() == nil || !t.packages[obj.Pkg().Path()] {
		return false
	}
	return t.reaches[memberKey(obj)]
}

func memberKey(obj *types.Func) string {
	key := obj.Pkg().Path() + "." + obj.Name()
	if recv := obj.Signature().Recv(); recv != nil {
		if named := receiver(recv.Type()); named != nil {
			key = obj.Pkg().Path() + "." + named.Obj().Name() + "." + obj.Name()
		}
	}
	return key
}

func writeReachFile(w io.Writer, dir string, chain toolchain) error {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.LoadAllSyntax,
		Dir:  dir,
		Env: append(os.Environ(),
			"GOOS="+chain.goos, "GOARCH="+chain.goarch, "CGO_ENABLED="+chain.cgo),
	}, "std")
	if err != nil {
		return err
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		return fmt.Errorf("%d packages of the standard library did not load", n)
	}
	prog, _ := ssautil.AllPackages(pkgs, 0)
	prog.Build()
	body := newBodyReach(nil)
	listed := map[string][]string{}
	for _, pkg := range pkgs {
		if !importable(pkg.PkgPath) {
			continue
		}
		var members []string
		for _, fn := range exportedMembers(prog, pkg.Types) {
			found, err := body.reaches(fn)
			if err != nil {
				return fmt.Errorf("%s: %w", fn, err)
			}
			if found {
				obj, isFunc := fn.Object().(*types.Func)
				assert.Ok(isFunc, "a reach file member has a declaration")
				members = append(members, strings.TrimPrefix(memberKey(obj), pkg.PkgPath+"."))
			}
		}
		slices.Sort(members)
		listed[pkg.PkgPath] = slices.Compact(members)
	}
	out := bufio.NewWriter(w)
	fmt.Fprintln(out, chain.key())
	for _, path := range slices.Sorted(maps.Keys(listed)) {
		fmt.Fprintln(out, path)
		for _, member := range listed[path] {
			fmt.Fprintf(out, "\t%s\n", member)
		}
	}
	fmt.Fprintln(out, reachFileEnd)
	return out.Flush()
}

func importable(path string) bool {
	if strings.HasPrefix(path, "cmd/") || strings.HasPrefix(path, "vendor/") {
		return false
	}
	return path != "internal" && !strings.HasPrefix(path, "internal/") &&
		!strings.Contains(path, "/internal/") && !strings.HasSuffix(path, "/internal")
}

func exportedMembers(prog *ssa.Program, pkg *types.Package) []*ssa.Function {
	var list []*ssa.Function
	add := func(obj *types.Func) {
		if fn := prog.FuncValue(obj); fn != nil {
			list = append(list, fn)
		}
	}
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		switch obj := obj.(type) {
		case *types.Func:
			add(obj)
		case *types.TypeName:
			if _, isAlias := obj.Type().(*types.Alias); isAlias {
				continue
			}
			named, ok := obj.Type().(*types.Named)
			if !ok || types.IsInterface(named) {
				continue
			}
			for _, recv := range []types.Type{named, types.NewPointer(named)} {
				set := types.NewMethodSet(recv)
				for i := 0; i < set.Len(); i++ {
					method, ok := set.At(i).Obj().(*types.Func)
					if ok && method.Exported() {
						add(method)
					}
				}
			}
		}
	}
	return list
}
